package cache

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
)

func UpdateInterfaceCache(c *gin.Context) {
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

	interfaceCacheJson, err := json.Marshal(timer.GetInterfaceCachesSnapshot())
	if err != nil {
		c.JSON(500, &gin.H{"error": "Failed to marshal interface cache"})
		return
	}
	clientSession.TrySend(&client.Message{
		Name:    "InterfaceCache",
		Content: string(interfaceCacheJson),
	})

	c.JSON(200, &gin.H{
		"success": true,
	})
}
