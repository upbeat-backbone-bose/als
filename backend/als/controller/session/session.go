package session

import (
	"context"
	"encoding/json"
	"log"
	"time"

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

func Handle(clientMgr *client.ClientManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := uuid.New().String()
		ctx, cancel := context.WithCancel(c.Request.Context())
		
		clientSession := client.NewClientSession(ctx, cancel)
		clientMgr.AddClient(uuid, clientSession)
		defer func() {
			cancel()
			clientMgr.RemoveClient(uuid)
		}()

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.SSEvent("SessionId", uuid)
		_config := &sessionConfig{
			ALSConfig: *config.Config,
			ClientIP:  c.ClientIP(),
		}

		configJson, err := json.Marshal(_config)
		if err != nil {
			log.Printf("Failed to marshal config: %v", err)
			return
		}
		c.SSEvent("Config", string(configJson))
		c.Writer.Flush()
		interfaceCacheJson, err := json.Marshal(timer.GetInterfaceCachesSnapshot())
		if err != nil {
			log.Printf("Failed to marshal interface cache: %v", err)
			return
		}
		c.SSEvent("InterfaceCache", string(interfaceCacheJson))
		c.Writer.Flush()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-clientSession.Channel:
				if !ok {
					return
				}
				c.SSEvent(msg.Name, msg.Content)
				c.Writer.Flush()
			}
		}
	}
}
