package als

import (
	"context"
	"log"
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

	if config.Config.FeatureIfaceTraffic {
		go timer.SetupInterfaceBroadcast()
	}
	go timer.UpdateSystemResource()
	go client.HandleQueue(context.Background())
	go cleanupExpiredClients()
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
