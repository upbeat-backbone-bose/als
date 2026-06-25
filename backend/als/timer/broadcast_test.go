package timer

import (
	"context"
	"testing"
	"time"
)

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

func TestInterfaceTrafficCacheDefaults(t *testing.T) {
	cache := &InterfaceTrafficCache{}
	if cache.InterfaceName != "" {
		t.Errorf("InterfaceName = %q; want empty", cache.InterfaceName)
	}
	if !cache.LastCacheTime.IsZero() {
		t.Errorf("LastCacheTime = %v; want zero", cache.LastCacheTime)
	}
	if cache.Caches != nil {
		t.Errorf("Caches = %v; want nil", cache.Caches)
	}
	if cache.LastRx != 0 || cache.LastTx != 0 {
		t.Errorf("counters = (%d,%d); want (0,0)", cache.LastRx, cache.LastTx)
	}
}

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

func TestGetInterfaceCachesSnapshotEmptyMap(t *testing.T) {
	withInterfaceCaches(t, map[int]*InterfaceTrafficCache{})

	got := GetInterfaceCachesSnapshot()
	if len(got) != 0 {
		t.Errorf("snapshot len = %d; want 0", len(got))
	}
}

func TestGetInterfaceCachesSnapshotNilMap(t *testing.T) {
	withInterfaceCaches(t, nil)

	got := GetInterfaceCachesSnapshot()
	if len(got) != 0 {
		t.Errorf("snapshot len = %d; want 0", len(got))
	}
}

func TestGetInterfaceCachesSnapshotMultipleEntries(t *testing.T) {
	now := time.Now()
	src := map[int]*InterfaceTrafficCache{
		1: {InterfaceName: "eth0", Caches: [][3]uint64{{1, 2, 3}}, LastCacheTime: now},
		2: {InterfaceName: "eth1", Caches: [][3]uint64{{4, 5, 6}}, LastCacheTime: now},
		3: {InterfaceName: "wlan0", Caches: [][3]uint64{{7, 8, 9}}, LastCacheTime: now},
	}
	withInterfaceCaches(t, src)

	got := GetInterfaceCachesSnapshot()
	if len(got) != 3 {
		t.Fatalf("snapshot len = %d; want 3", len(got))
	}
	for idx := 1; idx <= 3; idx++ {
		if _, ok := got[idx]; !ok {
			t.Errorf("idx %d missing from snapshot", idx)
		}
	}
}
