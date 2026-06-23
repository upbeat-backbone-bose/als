package iperf3

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

// safeChannelSend writes msg to ch without blocking. Returns true on
// success, false if the channel was full or ctx is cancelled. Mirrors
// the helper in als/controller/speedtest so both controllers share
// the same "drop on slow consumer" behaviour.
func safeChannelSend(ctx context.Context, ch chan<- *client.Message, msg *client.Message) bool {
	if ch == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case ch <- msg:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func randomPort(min, max int) (int, error) {
	if max < min {
		return 0, fmt.Errorf("invalid port range")
	}
	rng := max - min + 1
	n, err := rand.Int(rand.Reader, big.NewInt(int64(rng)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + min, nil
}

func Handle(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)

	timeout := time.Second * 60
	port, err := randomPort(config.Config.Iperf3StartPort, config.Config.Iperf3EndPort)
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(clientSession.GetContext(c.Request.Context()), timeout)
	defer cancel()

	// Send the assigned port to the client. Use non-blocking send so a
	// disconnected/slow SSE consumer cannot prevent the iperf3 server
	// from starting.
	if !safeChannelSend(ctx, clientSession.Channel, &client.Message{
		Name:    "Iperf3",
		Content: strconv.Itoa(port),
	}) && ctx.Err() != nil {
		c.JSON(500, gin.H{"success": false, "error": "client disconnected"})
		return
	}

	cmd := exec.CommandContext(ctx, "iperf3", "-s", "--forceflush", "-p", fmt.Sprintf("%d", port)) // #nosec G204 args are internally generated

	writer := func(pipe io.ReadCloser) {
		for {
			buf := make([]byte, 1024)
			n, err := pipe.Read(buf)
			if err != nil {
				return
			}
			msg := &client.Message{
				Name:    "Iperf3Stream",
				Content: string(buf[:n]),
			}
			// Non-blocking send so a slow consumer cannot block iperf3
			// from completing.
			if !safeChannelSend(ctx, clientSession.Channel, msg) {
				if ctx.Err() != nil {
					return
				}
			}
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}

	err = cmd.Start()
	if err != nil {
		c.JSON(400, &gin.H{
			"success": false,
		})
		return
	}

	go writer(stdoutPipe)
	go writer(stderrPipe)

	if err := cmd.Wait(); err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(200, &gin.H{
		"success": true,
	})
}
