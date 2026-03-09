package cache

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
)

func UpdateInterfaceCache(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)

	interfaceCacheJson, _ := json.Marshal(timer.GetInterfaceCachesSnapshot())
	clientSession.TrySend(&client.Message{
		Name:    "InterfaceCache",
		Content: string(interfaceCacheJson),
	})

	c.JSON(200, &gin.H{
		"success": true,
	})
}
