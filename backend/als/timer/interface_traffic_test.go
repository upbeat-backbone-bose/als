package timer

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// withInterfaceCaches replaces the global map for the duration of t.
func withInterfaceCaches(t *testing.T, m map[int]*InterfaceTrafficCache) {
	t.Helper()
	prev := InterfaceCaches
	interfaceCachesMu.Lock()
	InterfaceCaches = m
	interfaceCachesMu.Unlock()
	t.Cleanup(func() {
		interfaceCachesMu.Lock()
		InterfaceCaches = prev
		interfaceCachesMu.Unlock()
	})
}

func TestGetInterfaceCachesSnapshotEmpty(t *testing.T) {
	withInterfaceCaches(t, map[int]*InterfaceTrafficCache{})

	got := GetInterfaceCachesSnapshot()
	if got == nil {
		t.Fatal("snapshot is nil")
	}
	if len(got) != 0 {
		t.Errorf("snapshot len = %d; want 0", len(got))
	}
}

func TestGetInterfaceCachesSnapshotCopiesEntries(t *testing.T) {
	now := time.Now()
	src := map[int]*InterfaceTrafficCache{
		1: {
			InterfaceName: "eth0",
			LastCacheTime: now,
			Caches:        [][3]uint64{{uint64(now.Unix()), 1024, 2048}},
			LastRx:        1024,
			LastTx:        2048,
		},
		2: {
			InterfaceName: "eth1",
			LastCacheTime: now,
			Caches:        [][3]uint64{{uint64(now.Unix()), 512, 256}},
			LastRx:        512,
			LastTx:        256,
		},
	}
	withInterfaceCaches(t, src)

	got := GetInterfaceCachesSnapshot()
	if len(got) != 2 {
		t.Fatalf("snapshot len = %d; want 2", len(got))
	}

	for idx, want := range src {
		entry, ok := got[idx]
		if !ok {
			t.Errorf("idx %d missing from snapshot", idx)
			continue
		}
		if entry.InterfaceName != want.InterfaceName {
			t.Errorf("InterfaceName = %q; want %q", entry.InterfaceName, want.InterfaceName)
		}
		if !reflect.DeepEqual(entry.Caches, want.Caches) {
			t.Errorf("Caches = %v; want %v", entry.Caches, want.Caches)
		}
		if entry.LastRx != want.LastRx || entry.LastTx != want.LastTx {
			t.Errorf("counters = (%d,%d); want (%d,%d)", entry.LastRx, entry.LastTx, want.LastRx, want.LastTx)
		}
	}

	// Mutating the snapshot must not affect the source map (deep copy).
	got[1].LastRx = 9999
	if src[1].LastRx != 1024 {
		t.Errorf("source map mutated through snapshot; got %d", src[1].LastRx)
	}
}

func TestGetInterfaceCachesSnapshotIndependentCaches(t *testing.T) {
	src := map[int]*InterfaceTrafficCache{
		1: {
			InterfaceName: "eth0",
			Caches:        [][3]uint64{{1, 2, 3}, {4, 5, 6}},
		},
	}
	withInterfaceCaches(t, src)

	got := GetInterfaceCachesSnapshot()
	got[1].Caches[0][0] = 999

	if src[1].Caches[0][0] != 1 {
		t.Errorf("source Caches mutated through snapshot; got %d", src[1].Caches[0][0])
	}
}

func TestGetInterfaceCachesSnapshotNilMap(t *testing.T) {
	withInterfaceCaches(t, nil)

	got := GetInterfaceCachesSnapshot()
	if len(got) != 0 {
		t.Errorf("snapshot len = %d; want 0", len(got))
	}
}

func TestSetupInterfaceBroadcastContextCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		SetupInterfaceBroadcastContext(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetupInterfaceBroadcastContext did not exit after cancel")
	}
}

func TestSetupInterfaceBroadcastPreCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		SetupInterfaceBroadcastContext(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetupInterfaceBroadcast did not exit when context was pre-canceled")
	}
}

// Note: InterfaceTrafficCache struct-level tests (defaults, field
// round-trip) were removed -- they tested the Go zero-value
// contract, not the timer package, and would pass for any struct
// definition.

func TestInterfaceTrafficCacheFields(t *testing.T) {
	now := time.Now()
	cache := &InterfaceTrafficCache{
		InterfaceName: "eth0",
		LastCacheTime: now,
		Caches:        [][3]uint64{{1, 2, 3}},
		LastRx:        100,
		LastTx:        200,
	}
	if cache.InterfaceName != "eth0" {
		t.Errorf("InterfaceName = %q; want eth0", cache.InterfaceName)
	}
	if cache.LastRx != 100 || cache.LastTx != 200 {
		t.Errorf("counters = (%d,%d); want (100,200)", cache.LastRx, cache.LastTx)
	}
}
