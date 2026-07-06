package iperf3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
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

	// Pin the deterministic 500 path. When the request context is
	// already cancelled, the session-derived ctx is also done, so
	// SafeChannelSend returns false (ctx.Done branch), the
	// && ctx.Err() != nil guard fires, and the handler returns
	// 500 "client disconnected" -- no cmd.Start runs, so the 400
	// branch is unreachable here.
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

	cancelledReqCtx, cancelReq := context.WithCancel(context.Background())
	cancelReq()
	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody).WithContext(cancelledReqCtx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500 (client disconnected); body = %s", w.Code, w.Body.String())
	}
}

// TestHandleSpawnsAndFailsWithoutIperf3 exercises the path where
// iperf3 binary is not installed. We shadow PATH with a temp dir
// that has no iperf3 binary so the test is deterministic regardless
// of host. The handler picks a port, sends to the session, then
// tries to spawn iperf3 which fails because the binary is missing.
func TestHandleSpawnsAndFailsWithoutIperf3(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
	}
	t.Cleanup(func() { config.Config = prev })

	// Shadow PATH with an empty dir so exec.LookPath("iperf3")
	// always fails for this test, regardless of host state.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

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

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
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
// We prepend the fake binary dir to PATH so the test is
// deterministic regardless of whether iperf3 is installed.
func TestHandleEndToEndWithFakeIperf3(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
	}
	t.Cleanup(func() { config.Config = prev })

	dir := t.TempDir()
	// Fake iperf3 that prints one byte then exits successfully.
	// writeFakeIperf3 emits a platform-appropriate script so
	// cmd.Run can actually exec it on every host: shebang on
	// POSIX, .cmd with echo + exit /b on Windows.
	writeFakeIperf3(t, dir)
	// Replace PATH with the dir that holds the fake. We can't
	// rely on prepend because if the host has a real iperf3
	// installed (and is on PATH), LookPath would still resolve
	// to ours -- but the real binary is harmless here, and
	// replacement is more robust: it guarantees the fake is the
	// only iperf3 visible, regardless of host setup. The dir
	// also must not contain an iperf.exe (Windows PATHEXT
	// prefers .EXE over .CMD), which writeFakeIperf3 ensures
	// by naming the file iperf3.cmd on Windows.
	t.Setenv("PATH", dir)

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

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
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

// writeFakeIperf3 drops a fake iperf3 binary into dir. The fake
// prints one byte to stdout and exits successfully, which is
// enough to satisfy the handler's "cmd.Wait returns" path. The
// script form differs per host OS:
//
//   - POSIX: a plain "iperf3" file with #!/bin/sh shebang. The
//     shell runs `echo x`, then `exit 0`.
//   - Windows: an "iperf3.cmd" file invoked by cmd.exe via
//     PATHEXT. `echo x` writes the byte; `exit /b 0` returns
//     success. The .cmd extension is required because cmd.exe
//     only runs files with script extensions from PATH.
//
// On Windows we must NOT also create an iperf3.exe in PATH --
// PATHEXT lists .EXE before .CMD, so exec.LookPath would resolve
// "iperf3" to a (non-PE) .exe that fails silently rather than
// to our .cmd. We rely on LookPath falling through PATHEXT.
func writeFakeIperf3(t *testing.T, dir string) {
	t.Helper()
	var name, script string
	if runtime.GOOS == "windows" {
		name = "iperf3.cmd"
		script = "@echo off\r\necho x\r\nexit /b 0\r\n"
	} else {
		name = "iperf3"
		script = "#!/bin/sh\necho x\nexit 0\n"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake iperf3: %v", err)
	}
}

func TestRandomPortInRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lo   int
		hi   int
	}{
		{"normal range", 30000, 31000},
		{"narrow range", 5000, 5001},
		{"single port", 8080, 8080},
		{"large range", 1, 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Run several iterations since randomPort uses crypto/rand.
			for i := 0; i < 100; i++ {
				port, err := randomPort(tt.lo, tt.hi)
				if err != nil {
					t.Fatalf("randomPort(%d, %d) error: %v", tt.lo, tt.hi, err)
				}
				if port < tt.lo || port > tt.hi {
					t.Errorf("randomPort(%d, %d) = %d; out of range", tt.lo, tt.hi, port)
				}
			}
		})
	}
}

func TestRandomPortInvalidRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lo   int
		hi   int
	}{
		{"max less than min", 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			port, err := randomPort(tt.lo, tt.hi)
			if err == nil {
				t.Errorf("randomPort(%d, %d) = %d; want error", tt.lo, tt.hi, port)
			}
		})
	}
}

// randomPort does not validate that the values are positive port
// numbers -- only that hi >= lo. Negative values are accepted but
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
	seeds := []struct{ lo, hi int }{
		{30000, 31000},
		{1, 65535},
		{0, 0},
		{100, 50},
		{-10, -1},
		{-100, 100},
	}
	for _, s := range seeds {
		f.Add(s.lo, s.hi)
	}

	f.Fuzz(func(t *testing.T, lo, hi int) {
		port, err := randomPort(lo, hi)
		if err != nil {
			return
		}
		if port < lo || port > hi {
			t.Errorf("randomPort(%d, %d) = %d; out of range [%d, %d]", lo, hi, port, lo, hi)
		}
	})
}

// TestHandleMissingSession covers the 500 path when no clientSession
// is set on the gin context (middleware not installed).
func TestHandleMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/probe", Handle)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500; body = %s", w.Code, w.Body.String())
	}
}
