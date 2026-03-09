package session

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
)

type sessionConfig struct {
	config.ALSConfig
	ClientIP string `json:"my_ip"`
}

func Handle(c *gin.Context) {
	uuid := uuid.New().String()
	// uuid := "1"
	channel := make(chan *client.Message, 64)
	clientSession := &client.ClientSession{Channel: channel}
	client.AddClient(uuid, clientSession)
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	clientSession.SetContext(ctx)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.SSEvent("SessionId", uuid)
	_config := &sessionConfig{
		ALSConfig: *config.Config,
		ClientIP:  c.ClientIP(),
	}

	configJson, _ := json.Marshal(_config)
	c.SSEvent("Config", string(configJson))
	c.Writer.Flush()
	interfaceCacheJson, _ := json.Marshal(timer.GetInterfaceCachesSnapshot())
	c.SSEvent("InterfaceCache", string(interfaceCacheJson))
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			goto FINISH
		case msg, ok := <-channel:
			if !ok {
				break
			}
			c.SSEvent(msg.Name, msg.Content)
			c.Writer.Flush()
		}
	}

FINISH:
	client.RemoveClient(uuid)
}
