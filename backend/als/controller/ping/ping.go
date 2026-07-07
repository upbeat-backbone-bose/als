package ping

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/go-ping"
)

func Handle(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession, ok := client.SessionFromContext(v)
	if !ok {
		c.JSON(500, &gin.H{"success": false, "error": "Invalid session"})
		return
	}
	ip, ok := c.GetQuery("ip")
	if !ok {
		c.JSON(400, &gin.H{
			"success": false,
			"error":   "Invalid IP Address",
		})
		return
	}

	p, err := ping.New(ip)
	if err != nil {
		c.JSON(400, &gin.H{
			"success": false,
			"error":   "Invalid IP Address",
		})
		return
	}

	p.Count = 10
	// We pass context.Background() rather than c.Request.Context():
	// gin cancels c.Request.Context() the moment this handler
	// returns, which would happen immediately after c.JSON(200)
	// below and cascade into the pinger's ctx, killing the run
	// after only the first packet. The session's own parent ctx
	// (installed by session.go via SetContext) is still tracked
	// by GetContext, so when the client disconnects the session
	// is removed and the pinger is cancelled as a side effect.
	ctx := clientSession.GetContext(context.Background())
	p.OnEvent = func(event *ping.PacketEvent, _ error) {
		content, err := json.Marshal(event)
		if err != nil {
			return
		}
		client.SafeChannelSend(ctx, clientSession.Channel, &client.Message{
			Name:    "Ping",
			Content: string(content),
		})
	}

	// Start blocks until Count packets are sent or ctx is cancelled, so
	// it must run in a goroutine. The returned ctx is owned by the
	// caller -- defer its cancel to release the watcher goroutine
	// inside GetContext promptly when the client disconnects.
	go func() {
		// Swallow panics from the pinger: the SSE consumer is gone
		// by the time ctx is cancelled, and a panic on the cleanup
		// path would crash the whole process.
		func() {
			defer func() { _ = recover() }()
			p.Start(ctx)
		}()
		// Emit a final PingEnd event carrying the packet statistic so
		// the SSE consumer (Ping.vue) knows when the run has finished
		// naturally. The HTTP 200 of /method/ping returns immediately
		// while the pinger runs in the background, so the frontend
		// cannot use that to decide when to stop listening.
		//
		// Best-effort: SafeChannelSend drops on a full channel. The
		// frontend still recovers via Stop or session disconnect.
		statContent, err := json.Marshal(p.GetStatistic())
		if err != nil {
			return
		}
		client.SafeChannelSend(ctx, clientSession.Channel, &client.Message{
			Name:    "PingEnd",
			Content: string(statContent),
		})
	}()

	c.JSON(200, &gin.H{
		"success": true,
	})
}
