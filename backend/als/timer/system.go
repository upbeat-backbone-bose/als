package timer

import (
	"context"
	"runtime"
	"strconv"
	"time"

	"github.com/samlm0/als/v2/als/client"
)

// updateInterval is the default interval at which UpdateSystemResource
// broadcasts memory usage. Override updateIntervalForTest to shorten
// it during testing.
var updateInterval = 5 * time.Second

// tickerFactoryForTest returns the channel the goroutine reads ticks
// from. When nil the production default is used.
var tickerFactoryForTest func() <-chan time.Time

// UpdateSystemResource periodically broadcasts the current memory
// usage to every client. It exits when ctx is cancelled.
func UpdateSystemResource() {
	ctx := context.Background()
	UpdateSystemResourceContext(ctx)
}

// UpdateSystemResourceContext is the cancellable variant used by
// tests and by callers that need to shut the goroutine down.
func UpdateSystemResourceContext(ctx context.Context) {
	var m runtime.MemStats

	var tickerC <-chan time.Time
	if tickerFactoryForTest != nil {
		tickerC = tickerFactoryForTest()
	} else {
		t := time.NewTicker(updateInterval)
		defer t.Stop()
		tickerC = t.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerC:
			runtime.ReadMemStats(&m)
			client.BroadCastMessage("MemoryUsage", strconv.FormatUint(m.Sys, 10))
		}
	}
}
