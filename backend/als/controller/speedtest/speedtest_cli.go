package speedtest

import (
	"context"
	"encoding/json"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func HandleSpeedtestDotNet(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession, ok := client.SessionFromContext(v)
	if !ok {
		c.JSON(500, &gin.H{"success": false, "error": "Invalid session"})
		return
	}
	nodeId, ok := c.GetQuery("node_id")
	if !ok {
		nodeId = ""
	}
	var closed atomic.Bool
	timeout := time.Second * 60

	ctx, cancel := context.WithTimeout(clientSession.GetContext(c.Request.Context()), timeout)
	defer cancel()
	defer func() {
		closed.Store(true)
	}()
	go func() {
		<-ctx.Done()
		closed.Store(true)
	}()
	client.WaitQueue(ctx, func() {
		pos, totalPos := client.GetQueuePositionByCtx(ctx)
		msg, err := json.Marshal(gin.H{"type": "queue", "pos": pos, "totalPos": totalPos})
		if err != nil {
			return
		}
		if closed.Load() {
			return
		}
		client.SafeChannelSend(ctx, clientSession.Channel, &client.Message{
			Name:    "SpeedtestStream",
			Content: string(msg),
		})
	})
	args := []string{"--accept-license", "-f", "jsonl"}
	if nodeId != "" {
		args = append(args, "-s", nodeId)
	}
	cmd := exec.CommandContext(ctx, "speedtest", args...)

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": "stdout pipe: " + err.Error()})
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": "stderr pipe: " + err.Error()})
		return
	}

	if err := cmd.Start(); err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}

	go client.PipeToChannel(ctx, stdoutPipe, clientSession.Channel, "SpeedtestStream", func() bool { return !closed.Load() })
	go client.PipeToChannel(ctx, stderrPipe, clientSession.Channel, "SpeedtestStream", func() bool { return !closed.Load() })

	if err := cmd.Wait(); err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, &gin.H{
		"success": true,
	})
}
