package iperf3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

// TestHandleEndToEndWithFakeIperf3 drops a fake iperf3 onto PATH and
// exercises the full handler path: random port selection, channel
// send, cmd.Start, writer goroutines, cmd.Wait, response. The fake
// binary writes a byte to stdout and exits, so cmd.Wait completes.
func TestHandleEndToEndWithFakeIperf3(t *testing.T) {
	if _, err := exec.LookPath("iperf3"); err == nil {
		t.Skip("iperf3 is installed; cannot reliably override PATH")
	}

	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
	}
	t.Cleanup(func() { config.Config = prev })

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "iperf3")
	// Fake iperf3 that prints one byte then exits successfully.
	script := "#!/bin/sh\necho x\nexit 0\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake iperf3: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

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

	// Handler should respond 200 because the fake iperf3 exits 0.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}

	// Verify the assigned port was streamed to the client.
	select {
	case msg := <-session.Channel:
		if msg.Name != "Iperf3" {
			t.Errorf("first message name = %q; want Iperf3", msg.Name)
		}
	case <-time.After(time.Second):
		t.Error("no Iperf3 port message on the session channel")
	}
}

func TestRandomPortInRange(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	port, err := randomPort(-10, -1)
	if err != nil {
		t.Errorf("randomPort(-10, -1) error: %v", err)
	}
	if port < -10 || port > -1 {
		t.Errorf("randomPort(-10, -1) = %d; out of range", port)
	}
}

func FuzzRandomPort(f *testing.F) {
	seeds := []struct{ min, max int }{
		{30000, 31000},
		{1, 65535},
		{0, 0},
		{100, 50},
		{-10, -1},
		{-100, 100},
	}
	for _, s := range seeds {
		f.Add(s.min, s.max)
	}

	f.Fuzz(func(t *testing.T, min, max int) {
		port, err := randomPort(min, max)
		if err != nil {
			return
		}
		if port < min || port > max {
			t.Errorf("randomPort(%d, %d) = %d; out of range [%d, %d]", min, max, port, min, max)
		}
	})
}
