package timer

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/vishvananda/netlink"
)

type InterfaceTrafficCache struct {
	InterfaceName string
	LastCacheTime time.Time
	Caches        [][3]uint64
	LastRx        uint64 `json:"-"`
	LastTx        uint64 `json:"-"`
}

const (
	maxCacheEntries = 64
	maxCacheLen     = 30
)

var (
	interfaceCachesMu sync.RWMutex
	InterfaceCaches   = make(map[int]*InterfaceTrafficCache)
)

func GetInterfaceCachesSnapshot() map[int]*InterfaceTrafficCache {
	interfaceCachesMu.RLock()
	defer interfaceCachesMu.RUnlock()

	result := make(map[int]*InterfaceTrafficCache, len(InterfaceCaches))
	for idx, cache := range InterfaceCaches {
		copiedCaches := make([][3]uint64, len(cache.Caches))
		copy(copiedCaches, cache.Caches)
		result[idx] = &InterfaceTrafficCache{
			InterfaceName: cache.InterfaceName,
			LastCacheTime: cache.LastCacheTime,
			Caches:        copiedCaches,
			LastRx:        cache.LastRx,
			LastTx:        cache.LastTx,
		}
	}
	return result
}

func SetupInterfaceBroadcast(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	knownInterfaces := make(map[int]bool)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			interfaces, err := net.Interfaces()
			if err != nil {
				continue
			}

			interfaceCachesMu.Lock()
			for idx := range knownInterfaces {
				if !containsInterfaceIdx(interfaces, idx) {
					delete(InterfaceCaches, idx)
					delete(knownInterfaces, idx)
				}
			}

			for _, iface := range interfaces {
				if iface.Flags&net.FlagUp == 0 {
					continue
				}
				if strings.HasPrefix(iface.Name, "docker") || iface.Name == "lo" ||
					strings.HasPrefix(iface.Name, "wt") || strings.HasPrefix(iface.Name, "veth") {
					continue
				}

				link, err := netlink.LinkByIndex(iface.Index)
				if err != nil {
					continue
				}
				stats := link.Attrs().Statistics
				if stats == nil {
					continue
				}

				now := time.Now()
				ts := now.Unix()
				if ts < 0 {
					ts = 0
				}

				cache, ok := InterfaceCaches[iface.Index]
				if !ok {
					if len(InterfaceCaches) >= maxCacheEntries {
						interfaceCachesMu.Unlock()
						client.BroadCastMessage(
							"InterfaceTraffic",
							iface.Name+","+strconv.FormatInt(ts, 10)+","+strconv.FormatUint(stats.RxBytes, 10)+","+strconv.FormatUint(stats.TxBytes, 10),
						)
						knownInterfaces[iface.Index] = true
						continue
					}
					InterfaceCaches[iface.Index] = &InterfaceTrafficCache{
						InterfaceName: iface.Name,
						LastCacheTime: now,
						Caches:        make([][3]uint64, 0),
						LastRx:        0,
						LastTx:        0,
					}
					cache = InterfaceCaches[iface.Index]
					knownInterfaces[iface.Index] = true
				}

				cache.LastRx = stats.RxBytes
				cache.LastTx = stats.TxBytes
				cache.Caches = append(cache.Caches, [3]uint64{uint64(ts), cache.LastRx, cache.LastTx})
				if len(cache.Caches) > maxCacheLen {
					cache.Caches = cache.Caches[len(cache.Caches)-maxCacheLen:]
				}
				cache.LastCacheTime = now
				interfaceCachesMu.Unlock()
				client.BroadCastMessage(
					"InterfaceTraffic",
					iface.Name+","+strconv.FormatInt(ts, 10)+","+strconv.FormatUint(cache.LastRx, 10)+","+strconv.FormatUint(cache.LastTx, 10),
				)
			}
			interfaceCachesMu.Unlock()
		}
	}
}

func containsInterfaceIdx(interfaces []net.Interface, idx int) bool {
	for _, iface := range interfaces {
		if iface.Index == idx {
			return true
		}
	}
	return false
}
