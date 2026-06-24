package ping

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func TestHandleMissingIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/ping", Handle)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestHandleInvalidIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/ping", Handle)

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestHandleValidIPReturnsOK(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("ICMP requires root or CAP_NET_RAW; skip in non-privileged environments")
	}

	gin.SetMode(gin.TestMode)

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 64),
		CreatedAt: time.Now(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/ping", Handle)

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=127.0.0.1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler returns 200 immediately; ping runs asynchronously.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}
