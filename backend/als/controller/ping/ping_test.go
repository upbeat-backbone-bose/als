package ping

import (
	"context"
	"encoding/json"
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

// TestHandleEmitsPingEndAfterRun pins the contract that the SSE
// consumer (Ping.vue) depends on: after the pinger finishes, a
// PingEnd event carrying the packet statistic is delivered to the
// client channel. The frontend uses PingEnd to decide when to stop
// listening; without it the listener is removed on HTTP 200 (which
// happens immediately) and no Ping frames can ever reach the UI.
//
// 192.0.2.0/24 (TEST-NET-1, RFC 5737) is not routable, so the kernel
// never replies. Every packet is a timeout, but the loop still
// iterates Count times and then returns from p.Start, at which point
// PingEnd is emitted. The 500ms ctx timeout is a safety net; the
// run normally finishes in ~10s (Count * 1s Interval) but timeouts
// cut it short on the test machine.
func TestHandleEmitsPingEndAfterRun(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/ping?ip=192.0.2.1", http.NoBody).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}

	// Drain the channel looking for a PingEnd frame. Some Ping
	// frames may also be present (one per packet the kernel
	// processed before ctx was cancelled), but PingEnd is the
	// marker we care about.
	var foundEnd bool
	deadline := time.After(5 * time.Second)
	for !foundEnd {
		select {
		case msg, ok := <-session.Channel:
			if !ok {
				t.Fatal("channel closed before PingEnd was emitted")
			}
			if msg.Name == "PingEnd" {
				foundEnd = true
				// Content must be a JSON object that decodes into
				// the statistic struct. We don't pin every field,
				// just that it's well-formed JSON with the
				// expected shape.
				var stat struct {
					SendCount     int `json:"send_count"`
					ReceivedCount int `json:"received_count"`
					LossedCount   int `json:"lossed_count"`
				}
				if err := json.Unmarshal([]byte(msg.Content), &stat); err != nil {
					t.Errorf("PingEnd content is not valid statistic JSON: %v\ncontent: %s", err, msg.Content)
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for PingEnd event")
		}
	}
}
