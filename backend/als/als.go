package als

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/controller/shell"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
	alsHttp "github.com/samlm0/als/v2/http"
)

// Test-only injection points. nil means "use the production default".
var (
	cleanupContext   context.Context
	cleanupInterval  time.Duration
	newTickerForTest <-chan time.Time
)

func Init() {
	aHttp := alsHttp.CreateServer()

	log.Default().Println("Listen on: " + config.Config.ListenHost + ":" + config.Config.ListenPort)
	aHttp.SetListen(config.Config.ListenHost + ":" + config.Config.ListenPort)

	// Build the WebSocket Origin allow-list once, after config is loaded,
	// so the fake shell endpoint refuses CSWSH and Host-header smuggling
	// before the first request is served. When AllowedOrigins is empty
	// AND no public address is configured, the list stays empty and the
	// shell is effectively disabled (every upgrade is rejected). This
	// is the safe default -- operators must opt in to remote shell.
	shell.InitAllowedOrigins(config.Config)
	if len(shell.AllowedOriginsSnapshot()) == 0 && config.Config.FeatureShell {
		log.Default().Println("WARN: WebSocket shell endpoint active but no allowed origins configured; " +
			"all WebSocket upgrades will be rejected. Set ALLOWED_ORIGINS or PUBLIC_IPV4/PUBLIC_IPV6/LISTEN_IP.")
	}

	SetupHttpRoute(aHttp.GetEngine())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Config.FeatureIfaceTraffic {
		go timer.SetupInterfaceBroadcastContext(ctx)
	}
	go timer.UpdateSystemResourceContext(ctx)
	go client.HandleQueue(ctx)
	go cleanupExpiredClients()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Default().Println("Shutting down server...")
		cancel()
		// Release callers parked in WaitQueue immediately, even if their
		// parent context (e.g. a 60s speedtest timeout) is still alive.
		// HandleQueue will also call this on its own ctx.Done, but the
		// goroutine may be blocked inside a notify callback.
		client.ShutdownQueue()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := aHttp.Shutdown(shutdownCtx); err != nil {
			log.Default().Printf("Server forced to shutdown: %v", err)
		}
	}()

	if err := aHttp.Start(); err != nil {
		log.Default().Printf("Server failed to start: %v", err)
	}
}

// cleanupExpiredClients polls every cleanupInterval (default 1h) and
// removes any client sessions whose age exceeds the package-defined
// expiry. It exits when ctx is cancelled.
//
// The interval and ticker factory are overridable in tests via
// cleanupInterval / newTickerForTest.
func cleanupExpiredClients() {
	ctx := cleanupContext
	if ctx == nil {
		ctx = context.Background()
	}
	interval := cleanupInterval
	if interval == 0 {
		interval = time.Hour
	}
	tickerC := newTickerForTest
	if tickerC == nil {
		t := time.NewTicker(interval)
		defer t.Stop()
		tickerC = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerC:
			removed := client.RemoveExpiredClients()
			if removed > 0 {
				log.Default().Printf("Cleaned up %d expired sessions\n", removed)
			}
		}
	}
}
