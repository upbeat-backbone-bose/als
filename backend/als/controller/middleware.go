package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func MiddlewareSessionOnHeader(clientMgr *client.ClientManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionId := c.GetHeader("session")
		clientSession, ok := clientMgr.GetClient(sessionId)
		if !ok {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid session",
			})
			c.Abort()
			return
		}
		c.Set("clientSession", clientSession)
		c.Next()
	}
}

func MiddlewareSessionOnUrl(clientMgr *client.ClientManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionId := c.Param("session")
		clientSession, ok := clientMgr.GetClient(sessionId)
		if !ok {
			c.JSON(400, &gin.H{
				"success": false,
				"error":   "Invalid session",
			})
			c.Abort()
			return
		}
		c.Set("clientSession", clientSession)
		c.Next()
	}
}
