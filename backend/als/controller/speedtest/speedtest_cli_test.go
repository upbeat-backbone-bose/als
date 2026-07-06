package speedtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

// TestHandleSpeedtestDotNetMissingSession covers the 500 path when
// no clientSession is set on the gin context.
func TestHandleSpeedtestDotNetMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/probe", HandleSpeedtestDotNet)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500; body = %s", w.Code, w.Body.String())
	}
}

// runQueueHandler starts the package-level queue handler in a
// goroutine and returns a stop function. The handler exits when
// stop is called. Tests that exercise the speedtest endpoint must
// have the queue running because the handler calls client.WaitQueue.
func runQueueHandler(t *testing.T) func() {
	t.Helper()
	client.ResetQueueForTest()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		client.HandleQueue(ctx)
	}()

	if !client.WaitForHandlerParked(2 * time.Second) {
		cancel()
		t.Fatal("queue handler did not reach parked state in time")
	}

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("queue handler did not exit in time")
		}
		client.ResetQueueForTest()
	}
}

// TestHandleSpeedtestDotNetSpawnFailsWithoutBinary verifies the
// 500 path when the speedtest binary is not on PATH. We shadow
// PATH with an empty directory so the test is deterministic
// regardless of host state.
func TestHandleSpeedtestDotNetSpawnFailsWithoutBinary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stop := runQueueHandler(t)
	defer stop()

	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 64),
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
	r.GET("/probe", HandleSpeedtestDotNet)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()

	// Bound the request with a deadline so a regression that
	// hangs the handler is caught.
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500; body = %s", w.Code, w.Body.String())
	}
}

// TestHandleSpeedtestDotNetSuccess exercises the full handler
// path with a fake speedtest binary. The fake prints no output
// and exits 0, so the handler should return 200.
func TestHandleSpeedtestDotNetSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stop := runQueueHandler(t)
	defer stop()

	dir := t.TempDir()
	// We want LookPath to find *our* fake binary and not the
	// host's speedtest, so we replace PATH with the dir that
	// contains the fake rather than prepending. writeFakeSpeedtest
	// emits a platform-appropriate script (ping.cmd on Windows,
	// a shebang script on POSIX) so c.Run can actually exec it.
	writeFakeSpeedtest(t, dir, false)
	t.Setenv("PATH", dir)

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 64),
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
	r.GET("/probe", HandleSpeedtestDotNet)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}
}

// TestHandleSpeedtestDotNetWithNodeID passes a node_id query
// parameter and verifies the handler accepts it. Uses a fake
// speedtest that records its args to a file so the test can
// assert -s NODE_ID appears in argv.
func TestHandleSpeedtestDotNetWithNodeID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stop := runQueueHandler(t)
	defer stop()

	dir := t.TempDir()
	// argsLog path is the same one writeFakeSpeedtest writes to.
	writeFakeSpeedtest(t, dir, true)
	t.Setenv("PATH", dir)
	argsLog := filepath.Join(dir, "args.log")

	session := &client.ClientSession{
		Channel:   make(chan *client.Message, 64),
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
	r.GET("/probe", HandleSpeedtestDotNet)

	req := httptest.NewRequest(http.MethodGet, "/probe?node_id=1234", http.NoBody)
	w := httptest.NewRecorder()

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %s", w.Code, w.Body.String())
	}

	logBytes, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("args.log not written: %v", err)
	}
	// contains() is the package-local []string helper. We
	// tokenise the single-line argv on whitespace so each token
	// becomes a slice element; -s and 1234 must each appear as
	// distinct tokens to confirm node_id was passed as a single
	// argument.
	tokens := bytesFields(string(logBytes))
	if !contains(tokens, "-s") || !contains(tokens, "1234") {
		t.Errorf("args.log = %q; want it to contain tokens '-s' and '1234'", logBytes)
	}
}

// bytesFields is a minimal whitespace split. Using the stdlib
// strings.Fields would require an extra import for one call.
func bytesFields(s string) []string {
	var out []string
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// writeFakeSpeedtest drops a fake speedtest binary into dir that
// records its arguments to <dir>/args.log (when recordArgs is
// true) and exits successfully. The script must be runnable on
// the host OS, so the form differs per platform:
//
//   - POSIX: a plain "speedtest" file with #!/bin/sh shebang.
//     Shell expansion turns "$@" into the quoted argument list.
//   - Windows: a "speedtest.cmd" file invoked by cmd.exe via
//     PATHEXT. %* expands to the full argument list. The .cmd
//     extension is required because cmd.exe only runs files
//     with script extensions from PATH.
//
// On Windows we must NOT also create a speedtest.exe in PATH --
// PATHEXT lists .EXE before .CMD, so exec.LookPath would resolve
// "speedtest" to a (non-PE) .exe that fails silently rather than
// to our .cmd. We rely on LookPath falling through PATHEXT.
func writeFakeSpeedtest(t *testing.T, dir string, recordArgs bool) {
	t.Helper()
	argsLog := filepath.Join(dir, "args.log")
	var name, script string
	if runtime.GOOS == "windows" {
		name = "speedtest.cmd"
		lines := []string{"@echo off"}
		if recordArgs {
			// %* is the entire argument list; > truncates so
			// successive runs don't accumulate. Quote the
			// path to survive spaces in t.TempDir().
			lines = append(lines, "echo %* > \""+argsLog+"\"")
		}
		lines = append(lines, "exit /b 0")
		script = strings.Join(lines, "\r\n") + "\r\n"
	} else {
		name = "speedtest"
		lines := []string{"#!/bin/sh"}
		if recordArgs {
			lines = append(lines, "echo \"$@\" > "+argsLog)
		}
		lines = append(lines, "exit 0")
		script = strings.Join(lines, "\n") + "\n"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake speedtest: %v", err)
	}
}
