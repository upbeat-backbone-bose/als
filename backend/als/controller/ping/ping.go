package ping

import (
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/go-ping"
)

func Handle(clientMgr *client.ClientManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip, queryOk := c.GetQuery("ip")
		v, sessionOk := c.Get("clientSession")
		if !sessionOk {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid session",
			})
			c.Abort()
			return
		}
		clientSession, ok := v.(*client.ClientSession)
		if !ok {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid session type",
			})
			c.Abort()
			return
		}
		
		if !queryOk {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid IP Address",
			})
			return
		}

		channel := clientSession.Channel

		p, err := ping.New(ip)
		if err != nil {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid IP Address",
			})
			return
		}

		p.Count = 10
		p.OnEvent = func(event *ping.PacketEvent, err error) {
			if err != nil {
				log.Printf("Ping error: %v", err)
				return
			}
			content, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				log.Printf("Failed to marshal ping event: %v", marshalErr)
				return
			}
			msg := &client.Message{
				Name:    "Ping",
				Content: string(content),
			}
			select {
			case channel <- msg:
			default:
				log.Println("Channel full, dropping ping event")
			}
		}
		go p.Start(c.Request.Context())

		c.JSON(200, &gin.H{
			"success": true,
		})
	}
}
