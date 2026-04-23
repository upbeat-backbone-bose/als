package speedtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func HandleSpeedtestDotNet(c *gin.Context) {
	nodeId, ok := c.GetQuery("node_id")
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)
	if !ok {
		nodeId = ""
	}
	var closed atomic.Bool
	timeout := time.Second * 60

	ctx, cancel := context.WithTimeout(clientSession.Context(), timeout)
	defer cancel()
	defer func() {
		closed.Store(true)
	}()
	go func() {
		<-ctx.Done()
		closed.Store(true)
	}()
	client.WaitQueue(ctx, func() {
		pos, totalPos := client.GetQueuePostitionByCtx(ctx)
		msg, _ := json.Marshal(gin.H{"type": "queue", "pos": pos, "totalPos": totalPos})
		if !closed.Load() {
			clientSession.Channel <- &client.Message{
				Name:    "SpeedtestStream",
				Content: string(msg),
			}
		}
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

	writer := func(pipe io.ReadCloser) {
		for {
			buf := make([]byte, 1024)
			n, err := pipe.Read(buf)
			if err != nil {
				return
			}
			if !closed.Load() {
				clientSession.Channel <- &client.Message{
					Name:    "SpeedtestStream",
					Content: string(buf[:n]),
				}
			}
		}
	}

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

	go writer(stdoutPipe)
	go writer(stderrPipe)

	if err := cmd.Wait(); err != nil {
		log.Printf("Speedtest command failed: %v", err)
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}
	log.Println("Speedtest completed successfully")
	c.JSON(200, &gin.H{
		"success": true,
	})
}
