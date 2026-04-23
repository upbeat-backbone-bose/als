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
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			interfaces, err := net.Interfaces()
			if err != nil {
				continue
			}

			for _, iface := range interfaces {
				// skip down interface
				if iface.Flags&net.FlagUp == 0 {
					continue
				}

				// skip docker
				if strings.HasPrefix(iface.Name, "docker") {
					continue
				}

				// skip lo
				if iface.Name == "lo" {
					continue
				}

				// skip wireguard
				if strings.HasPrefix(iface.Name, "wt") {
					continue
				}

				// skip veth
				if strings.HasPrefix(iface.Name, "veth") {
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
				interfaceCachesMu.Lock()
				cache, ok := InterfaceCaches[iface.Index]
				if !ok {
					InterfaceCaches[iface.Index] = &InterfaceTrafficCache{
						InterfaceName: iface.Name,
						LastCacheTime: now,
						Caches:        make([][3]uint64, 0),
						LastRx:        0,
						LastTx:        0,
					}
					cache = InterfaceCaches[iface.Index]
				}

				cache.LastRx = stats.RxBytes
				cache.LastTx = stats.TxBytes

				cache.Caches = append(cache.Caches, [3]uint64{uint64(ts), cache.LastRx, cache.LastTx})
				if len(cache.Caches) > 30 {
					cache.Caches = cache.Caches[len(cache.Caches)-30:]
				}
				cache.LastCacheTime = now
				interfaceCachesMu.Unlock()
				client.BroadCastMessage(
					"InterfaceTraffic",
					iface.Name+","+strconv.FormatInt(ts, 10)+","+strconv.FormatUint(cache.LastRx, 10)+","+strconv.FormatUint(cache.LastTx, 10),
				)
			}
		}
	}
}
