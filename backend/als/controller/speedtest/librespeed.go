package speedtest

import (
	"crypto/rand"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	fixedChunkSize = 65536
	maxTotalSize   = 104857600
)

func HandleDownload(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusOK)

	remaining := maxTotalSize
	for remaining > 0 {
		size := fixedChunkSize
		if remaining < fixedChunkSize {
			size = remaining
		}
		data := make([]byte, size)
		if _, err := rand.Read(data); err != nil {
			return
		}
		if _, err := c.Writer.Write(data); err != nil {
			return
		}
		remaining -= size
	}
}

func HandleUpload(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, s-maxage=0, post-check=0, pre-check=0")
	c.Header("Pragma", "no-cache")
	c.Header("Connection", "keep-alive")
	_, err := io.Copy(io.Discard, c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
}
