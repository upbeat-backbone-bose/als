package ping

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/go-ping"
)

func Handle(c *gin.Context) {
	ip, ok := c.GetQuery("ip")
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)
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
	ctx := clientSession.GetContext(c.Request.Context())
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
	p.Start(ctx)

	c.JSON(200, &gin.H{
		"success": true,
	})
}
