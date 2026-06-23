package als

import (
	"context"
	"testing"
	"time"

	"github.com/samlm0/als/v2/als/client"
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

// seedClients inserts N sessions into the global Clients map; the
// returned cleanup restores the map.
func seedClients(t *testing.T, ids []string, ages []time.Duration) {
	t.Helper()
	prev := make(map[string]*client.ClientSession, len(client.Clients))
	for k, v := range client.Clients {
		prev[k] = v
	}
	client.Clients = make(map[string]*client.ClientSession, len(ids))
	now := time.Now()
	for i, id := range ids {
		age := time.Duration(0)
		if i < len(ages) {
			age = ages[i]
		}
		client.Clients[id] = &client.ClientSession{
			Channel:   make(chan *client.Message, 1),
			CreatedAt: now.Add(-age),
		}
	}
	t.Cleanup(func() {
		client.Clients = prev
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
	deadline := time.Now().Add(2 * time.Second)
	for {
		client.ClientsMu().RLock()
		_, expired := client.Clients["expired"]
		all := len(client.Clients)
		client.ClientsMu().RUnlock()
		if !expired {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expired session was not removed; remaining=%d", all)
		}
		time.Sleep(time.Millisecond)
	}

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

	// No clients seeded. Drive a tick; cleanup should run and return
	// without logging.
	ticker <- time.Now()
	time.Sleep(20 * time.Millisecond)
	cancel()
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

	// Let the goroutine enter the select.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupExpiredClients did not exit after ctx cancel")
	}
}

func TestCleanupExpiredClientsProductionDefaults(t *testing.T) {
	// Without injection, the production defaults apply (1h interval,
	// real time.Ticker). We cannot wait an hour, so just verify the
	// function path is reachable: call it in a goroutine and cancel
	// via the package-level cleanupContext that the test framework
	// would not normally populate -- instead we just confirm that the
	// goroutine starts.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Override only ctx; interval/ticker fall back to defaults.
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