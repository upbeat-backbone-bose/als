package iperf3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

func TestHandleInvalidPortRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 100,
		Iperf3EndPort:   50, // invalid: max < min
	}
	t.Cleanup(func() { config.Config = prev })

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/probe", Handle)

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", w.Code)
	}
}

func TestHandleClientDisconnected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
	}
	t.Cleanup(func() { config.Config = prev })

	// Channel already full. We use a request whose context is already
	// cancelled; the handler creates a derived ctx that is also
	// immediately done, so SafeChannelSend will hit the ctx.Done
	// branch and return false.
	full := make(chan *client.Message, 1)
	full <- &client.Message{Name: "filler"}
	parentCtx, pcancel := context.WithCancel(context.Background())
	pcancel()
	session := &client.ClientSession{
		Channel:   full,
		CreatedAt: time.Now(),
	}
	session.SetContext(parentCtx)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/probe", Handle)

	cancelledReqCtx, c := context.WithCancel(context.Background())
	c()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil).WithContext(cancelledReqCtx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Both outcomes are acceptable: SafeChannelSend returned false
	// because the derived ctx was already done (handler returns 500),
	// or because the channel was full and the derived ctx was not
	// done (handler proceeds to cmd.Start which fails -> 400).
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 500 or 400; body = %s", w.Code, w.Body.String())
	}
}

// TestHandleSpawnsAndFailsWithoutIperf3 exercises the path where
// iperf3 binary is not installed. The handler picks a port, sends
// to the session, then tries to spawn iperf3 which fails because
// the binary is missing. We assert the handler returns the expected
// error code.
func TestHandleSpawnsAndFailsWithoutIperf3(t *testing.T) {
	if _, err := exec.LookPath("iperf3"); err == nil {
		t.Skip("iperf3 is installed; cannot exercise the spawn-failure path")
	}

	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
	}
	t.Cleanup(func() { config.Config = prev })

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 4),
		CreatedAt: time.Now(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session.SetContext(ctx)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("clientSession", session)
		c.Next()
	})
	r.GET("/probe", Handle)

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Without iperf3, cmd.Start fails -> 400.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestRandomPortInRange(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
	}{
		{"normal range", 30000, 31000},
		{"narrow range", 5000, 5001},
		{"single port", 8080, 8080},
		{"large range", 1, 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run several iterations since randomPort uses crypto/rand.
			for i := 0; i < 100; i++ {
				port, err := randomPort(tt.min, tt.max)
				if err != nil {
					t.Fatalf("randomPort(%d, %d) error: %v", tt.min, tt.max, err)
				}
				if port < tt.min || port > tt.max {
					t.Errorf("randomPort(%d, %d) = %d; out of range", tt.min, tt.max, port)
				}
			}
		})
	}
}

func TestRandomPortInvalidRange(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
	}{
		{"max less than min", 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := randomPort(tt.min, tt.max)
			if err == nil {
				t.Errorf("randomPort(%d, %d) = %d; want error", tt.min, tt.max, port)
			}
		})
	}
}

// randomPort does not validate that the values are positive port
// numbers -- only that max >= min. Negative values are accepted but
// cannot be opened. We document the current behaviour here.
func TestRandomPortAcceptsNegativeRange(t *testing.T) {
	port, err := randomPort(-10, -1)
	if err != nil {
		t.Errorf("randomPort(-10, -1) error: %v", err)
	}
	if port < -10 || port > -1 {
		t.Errorf("randomPort(-10, -1) = %d; out of range", port)
	}
}