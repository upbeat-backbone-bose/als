package speedtest

import (
	"crypto/rand"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	maxChunks       = 1024
	maxChunkSize    = 1048576
	maxTotalSize    = 104857600
)

func HandleDownload(c *gin.Context) {
	chunks := 4
	if ckSize, ok := c.GetQuery("ckSize"); ok {
		if ckSizeInt, err := strconv.Atoi(ckSize); err == nil && ckSizeInt > 0 {
			chunks = ckSizeInt
			if chunks > maxChunks {
				chunks = maxChunks
			}
		}
	}

	chunkSize := maxChunkSize
	if cs, ok := c.GetQuery("cs"); ok {
		if csInt, err := strconv.Atoi(cs); err == nil && csInt > 0 {
			chunkSize = csInt
			if chunkSize > maxChunkSize {
				chunkSize = maxChunkSize
			}
		}
	}

	totalSize := chunks * chunkSize
	if totalSize > maxTotalSize {
		totalSize = maxTotalSize
		chunks = maxTotalSize / chunkSize
	}

	data := make([]byte, chunkSize)
	if _, err := rand.Read(data); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Writer.WriteHeader(http.StatusOK)
	for i := 0; i < chunks; i++ {
		if _, err := c.Writer.Write(data); err != nil {
			return
		}
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
