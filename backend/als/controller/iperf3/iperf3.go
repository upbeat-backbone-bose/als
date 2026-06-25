package iperf3

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

func randomPort(lo, hi int) (int, error) {
	if hi < lo {
		return 0, fmt.Errorf("invalid port range")
	}
	rng := hi - lo + 1
	n, err := rand.Int(rand.Reader, big.NewInt(int64(rng)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + lo, nil
}

func Handle(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession, ok := client.SessionFromContext(v)
	if !ok {
		c.JSON(500, &gin.H{"success": false, "error": "Invalid session"})
		return
	}

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
	// from starting. If the client is already gone (ctx done), fail
	// fast rather than spawning iperf3 for a consumer that cannot
	// read its output.
	client.SafeChannelSend(ctx, clientSession.Channel, &client.Message{
		Name:    "Iperf3",
		Content: strconv.Itoa(port),
	})
	if ctx.Err() != nil {
		c.JSON(500, gin.H{"success": false, "error": "client disconnected"})
		return
	}

	cmd := exec.CommandContext(ctx, "iperf3", "-s", "--forceflush", "-p", strconv.Itoa(port)) // #nosec G204 args are internally generated

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

	go client.PipeToChannel(ctx, stdoutPipe, clientSession.Channel, "Iperf3Stream", nil)
	go client.PipeToChannel(ctx, stderrPipe, clientSession.Channel, "Iperf3Stream", nil)

	if err := cmd.Wait(); err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(200, &gin.H{
		"success": true,
	})
}
