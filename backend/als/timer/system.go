package timer

import (
	"context"
	"runtime"
	"strconv"
	"time"

	"github.com/samlm0/als/v2/als/client"
)

func UpdateSystemResource(ctx context.Context) {
	var m runtime.MemStats
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.ReadMemStats(&m)
			client.BroadCastMessage("MemoryUsage", strconv.FormatUint(m.Sys, 10))
		}
	}
}
