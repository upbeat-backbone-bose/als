package als

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samlm0/als/v2/als/client"
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

	aHttp.Start()
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
