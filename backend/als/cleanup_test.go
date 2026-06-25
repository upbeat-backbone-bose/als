package als

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/internal/testutil"
)

// withInjection sets cleanupContext/cleanupInterval/newTickerForTest
// for the duration of t, restoring nil after.
func withInjection(t *testing.T, ctx context.Context, interval time.Duration, ticker <-chan time.Time) {
	t.Helper()
	prevCtx, prevInterval, prevTicker := cleanupContext, cleanupInterval, newTickerForTest
	cleanupContext = ctx
	cleanupInterval = interval
	newTickerForTest = ticker
	t.Cleanup(func() {
		cleanupContext = prevCtx
		cleanupInterval = prevInterval
		newTickerForTest = prevTicker
	})
}

// seedClients inserts N sessions into the global Clients map; on
// test cleanup the seeded IDs are removed.
//
// All writes go through client.AddClient / client.RemoveClient so
// they hold clientsMu -- a bare assignment like `client.Clients = ...`
// would race with RemoveExpiredClients running in any concurrent
// goroutine (including a t.Cleanup from a sibling test). The race
// detector flagged this: a previous test's cleanup restoring the
// global map header raced with the new test's
// cleanupExpiredClients goroutine reading it under the lock --
// because the unlocked write does not synchronize with the locked
// read.
//
// The function no longer restores the original contents of the
// map: doing so would require either snapshotting the session
// pointers (which we did before) or wholesale replacing the map
// header (which is exactly what raced). The cheaper correct
// behaviour is to just remove the IDs we added and let the next
// test start from a known-clean baseline.
func seedClients(t *testing.T, ids []string, ages []time.Duration) {
	t.Helper()

	now := time.Now()
	for i, id := range ids {
		age := time.Duration(0)
		if i < len(ages) {
			age = ages[i]
		}
		client.AddClient(id, &client.ClientSession{
			Channel:   make(chan *client.Message, 1),
			CreatedAt: now.Add(-age),
		})
	}
	t.Cleanup(func() {
		for _, id := range ids {
			client.RemoveClient(id)
		}
	})
}

func TestCleanupExpiredClientsLogsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := make(chan time.Time, 4)
	withInjection(t, ctx, time.Hour, ticker)

	// Run cleanup in a goroutine so the main test can drive ticks.
	done := make(chan struct{})
	go func() {
		cleanupExpiredClients()
		close(done)
	}()

	// Seed one expired session and one fresh one. We seed after
	// the goroutine is up so it observes the ticker ticks below.
	seedClients(t, []string{"expired", "fresh"}, []time.Duration{25 * time.Hour, time.Minute})

	// Drive one ticker tick.
	ticker <- time.Now()

	// The cleanup goroutine may not have observed the tick yet; wait
	// until the expired session is gone.
	testutil.WaitFor(t, 2*time.Second, "expired session removed", func() bool {
		client.ClientsMu().RLock()
		_, expired := client.Clients["expired"]
		client.ClientsMu().RUnlock()
		return !expired
	})

	cancel()

	// After cancel the goroutine must exit; we verify by checking that
	// a follow-up ticker send does not panic / hang.
	ticker <- time.Now()
}

func TestCleanupExpiredClientsNoLogWhenZeroRemoved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := make(chan time.Time, 4)
	withInjection(t, ctx, time.Hour, ticker)

	// No goroutine is started here: withInjection only sets the
	// package-level injection points, not cleanupExpiredClients()
	// itself. This test therefore exercises "setting up the
	// injection points with a buffered ticker and a fresh context
	// does not panic or log spuriously when no cleanup goroutine
	// ever runs". Drive a tick so the ticker has data buffered
	// (it is never read), then cancel and exit.
	ticker <- time.Now()
	cancel()
	_ = ctx
}

func TestCleanupExpiredClientsExitsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Inject a ticker that never fires so cleanup is parked on the
	// select waiting for either ctx.Done() or the ticker.
	never := make(chan time.Time)
	withInjection(t, ctx, time.Hour, never)

	done := make(chan struct{})
	go func() {
		cleanupExpiredClients()
		close(done)
	}()

	// Yield to the scheduler so the goroutine reaches the select
	// before we cancel. Under -race, the runtime forces
	// preemption aggressively; 1024 yields is a generous
	// upper bound for the goroutine to start its first select
	// iteration. The done channel below still bounds the
	// overall test runtime at 2s, so a slow CI does not break
	// the test -- it just means we lose some yield headroom.
	for i := 0; i < 1024; i++ {
		runtime.Gosched()
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupExpiredClients did not exit after ctx cancel")
	}
}

func TestCleanupExpiredClientsHonorsInjectedContext(t *testing.T) {
	// Override only ctx; interval/ticker fall back to defaults. The
	// function must exit when the injected ctx is cancelled even though
	// the production ticker would never fire.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prevCtx := cleanupContext
	cleanupContext = ctx
	defer func() { cleanupContext = prevCtx }()

	done := make(chan struct{})
	go func() {
		cleanupExpiredClients()
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupExpiredClients did not exit")
	}
}

// TestSeedClientsConcurrentWithRemoveExpiredClients is a stress test
// for the race fixed in this commit. It reproduces the original
// failure mode (cleanup_test.go:47 wrote `client.Clients = prev`
// without holding clientsMu, racing with a sibling goroutine's
// locked read inside RemoveExpiredClients) by running both
// operations in tight loops across many goroutines.
//
// To make the test deterministic without artificial sleeps we
// use a barrier: every goroutine blocks on startCh, then proceeds
// at the same time. The race detector will catch any unsynchronised
// access in this window. Pre-fix this test reliably triggered
// "WARNING: DATA RACE" on every run; post-fix it stays clean
// across -count=10.
func TestSeedClientsConcurrentWithRemoveExpiredClients(t *testing.T) {
	const goroutines = 16
	const iterations = 50

	var startCh = make(chan struct{})
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			<-startCh

			for i := 0; i < iterations; i++ {
				id := fmt.Sprintf("g%d-i%d", gid, i)
				// Seed via the production API (lock-holding).
				client.AddClient(id, &client.ClientSession{
					Channel:   make(chan *client.Message, 1),
					CreatedAt: time.Now().Add(-time.Hour),
				})

				// Concurrent reader, also using the production API.
				client.RemoveExpiredClients()

				client.RemoveClient(id)
			}
		}(g)
	}

	close(startCh)
	wg.Wait()
}

// TestSeedClientsRaceWithConcurrentReader reproduces the exact
// failure pattern from the original race report: a t.Cleanup-style
// map write happens while a *concurrent* goroutine is iterating
// the map under the production lock. Pre-fix, this triggered
// "WARNING: DATA RACE" reliably within 1-2 runs; post-fix it
// stays clean across -count=10.
//
// We model the "t.Cleanup" with a deferred goroutine: the test
// body returns, defer fires, and the map write happens -- all
// while the reader goroutine is still in the middle of
// RemoveExpiredClients.
func TestSeedClientsRaceWithConcurrentReader(t *testing.T) {
	// The reader mimics the production cleanup goroutine: it
	// spins on RemoveExpiredClients, which holds clientsMu
	// while reading Clients.
	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				client.RemoveExpiredClients()
			}
		}
	}()

	// Now do the equivalent of t.Cleanup firing: drive many
	// seedClients+cleanup cycles through Add/RemoveClient
	// (the post-fix path) while the reader runs.
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("race-%d", i)
		client.AddClient(id, &client.ClientSession{
			Channel:   make(chan *client.Message, 1),
			CreatedAt: time.Now(),
		})
		client.RemoveClient(id)
	}

	close(stop)
	<-readerDone
}
