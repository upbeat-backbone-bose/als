package cache

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
)

func UpdateInterfaceCache(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession, ok := client.SessionFromContext(v)
	if !ok {
		c.JSON(500, &gin.H{"success": false, "error": "Invalid session"})
		return
	}

	interfaceCacheJson, err := json.Marshal(timer.GetInterfaceCachesSnapshot())
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
		return
	}
	if !clientSession.TrySend(&client.Message{
		Name:    "InterfaceCache",
		Content: string(interfaceCacheJson),
	}) {
		// Client is not keeping up or already gone; surface the failure
		// rather than silently dropping the message.
		c.JSON(503, &gin.H{"success": false, "error": "client not ready"})
		return
	}

	c.JSON(200, &gin.H{
		"success": true,
	})
}
