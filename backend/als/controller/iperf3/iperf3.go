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

func Handle(clientMgr *client.ClientManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("clientSession")
		if !ok {
			c.JSON(400, &gin.H{"error": "Invalid session"})
			c.Abort()
			return
		}
		clientSession, ok := v.(*client.ClientSession)
		if !ok {
			c.JSON(400, &gin.H{"error": "Invalid session type"})
			c.Abort()
			return
		}

		timeout := time.Second * 60
		port, err := randomPort(config.Config.Iperf3StartPort, config.Config.Iperf3EndPort)
		if err != nil {
			c.JSON(500, &gin.H{"success": false, "error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(clientSession.Context(), timeout)
		defer cancel()

		clientMgr.WaitQueue(ctx, nil)

		cmd := exec.CommandContext(ctx, "iperf3", "-s", "--forceflush", "-p", fmt.Sprintf("%d", port))
		clientSession.Channel <- &client.Message{
			Name:    "Iperf3",
			Content: strconv.Itoa(port),
		}

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
				clientSession.Channel <- msg
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
			c.JSON(500, &gin.H{
				"success": false,
				"error":   err.Error(),
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
}
