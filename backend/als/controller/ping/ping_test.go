package ping

import (
	"context"
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

	req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=invalid", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=127.0.0.1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler returns 200 immediately; ping runs asynchronously.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

// TestHandleMissingSession covers the 500 path when no clientSession
// is set on the gin context (middleware not installed).
func TestHandleMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/ping", Handle)

	req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500; body = %s", w.Code, w.Body.String())
	}
}

// TestHandleReturnsImmediatelyWithoutWaitingForPing pins the
// behaviour that the ping goroutine does not block the HTTP response.
// Regression: previously p.Start was called synchronously, so the
// handler would not return 200 until the ping loop completed (Count
// packets) or ctx was cancelled -- minutes under typical conditions.
//
// We use 192.0.2.0/24 (TEST-NET-1, RFC 5737): reserved for
// documentation, not routable, so the kernel will not respond.
// ping.New("192.0.2.1") succeeds (parses as a valid IP), but
// p.Start would block until ctx cancels.
func TestHandleReturnsImmediatelyWithoutWaitingForPing(t *testing.T) {
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

	// Use a request context that auto-cancels after 200ms so the
	// background ping goroutine is bounded even if the test asserts
	// fail.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=192.0.2.1", http.NoBody).WithContext(ctx)
	w := httptest.NewRecorder()

	// If p.Start were called synchronously, this call would block
	// until ctx times out. The handler must return well before that.
	start := time.Now()
	r.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("handler took %v; want < 100ms (regression: p.Start is blocking the handler)", elapsed)
	}
}
