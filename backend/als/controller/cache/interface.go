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
		c.JSON(400, &gin.H{"success": false, "error": "Invalid session"})
		return
	}
	clientSession := v.(*client.ClientSession)

	interfaceCacheJson, err := json.Marshal(timer.GetInterfaceCachesSnapshot())
	if err != nil {
		c.JSON(500, &gin.H{"success": false, "error": err.Error()})
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
