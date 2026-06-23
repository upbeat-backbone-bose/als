package speedtest

import (
	"crypto/rand"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/config"
)

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func sizeToBytes(size string) (int64, error) {
	re := regexp.MustCompile(`^(\d+)(KB|MB|GB|TB)$`)
	matches := re.FindStringSubmatch(size)

	if matches == nil {
		return 0, fmt.Errorf("invalid size format")
	}

	num, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	switch strings.ToUpper(matches[2]) {
	case "KB":
		num *= 1024
	case "MB":
		num *= 1024 * 1024
	case "GB":
		num *= 1024 * 1024 * 1024
	case "TB":
		num *= 1024 * 1024 * 1024 * 1024
	}

	// Reject zero: the streaming handler emits Content-Length: 0 and the
	// downstream speed measurement would divide by zero. 0KB.test is a
	// degenerate request with no useful behavior.
	if num <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}

	return num, nil
}

// HandleFakeFile streams random bytes to the response to back the
// client-side speedtest. The handler is registered under the
// /session/:session group but does NOT call WaitQueue: this endpoint
// has no shared state that needs FIFO serialisation, and the queue
// handler is only running for speedtest-cli / iperf3. Calling
// WaitQueue here would block forever.
func HandleFakeFile(c *gin.Context) {
	filename := c.Param("filename")
	re := regexp.MustCompile(`^(\d+)(KB|MB|GB|TB)\.test$`)

	pos := re.FindStringIndex(filename)
	if pos == nil {
		c.String(404, "404 file not found")
		return
	}

	filename = filename[0 : len(filename)-5]
	if !contains(config.Config.SpeedtestFileList, filename) {
		c.String(404, "404 file not found")
		return
	}

	size, err := sizeToBytes(filename)
	if err != nil {
		c.String(404, "Invalid file size")
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 1024*1024)
		if _, err := rand.Read(buf); err != nil {
			return false
		}

		for size > 0 {
			if size < int64(len(buf)) {
				buf = buf[:size]
			}

			if _, err := w.Write(buf); err != nil {
				return false
			}

			size -= int64(len(buf))
		}

		return false
	})
}
