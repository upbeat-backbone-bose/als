package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// waitFor spins until cond returns true or timeout elapses. Replaces
// the legacy time.Sleep(time.Millisecond) pattern. See als/client/wait_helpers_test.go
// for the canonical implementation; this is duplicated to keep
// the test packages independent.
//
// linter's per-file analysis can't see across files.
//
//nolint:unparam // each call site picks a different timeout; the
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitFor timed out after %v: %s", timeout, msg)
		}
		select {
		case <-tick.C:
		default:
			runtime.Gosched()
		}
	}
}

func TestCreateServer(t *testing.T) {
	s := CreateServer()
	if s == nil {
		t.Fatal("CreateServer returned nil")
	}
	if s.engine == nil {
		t.Error("engine is nil")
	}
	if s.listen != ":8080" {
		t.Errorf("listen = %q; want :8080", s.listen)
	}
}

func TestGetEngine(t *testing.T) {
	s := CreateServer()
	e := s.GetEngine()
	if e == nil {
		t.Fatal("GetEngine returned nil")
	}

	// Register a route through the returned engine, then dispatch a
	// request. This proves GetEngine returns the live engine.
	e.GET("/probe", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200; body = %q", w.Code, w.Body.String())
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q; want ok", w.Body.String())
	}
}

func TestSetListen(t *testing.T) {
	s := CreateServer()
	s.SetListen("127.0.0.1:9999")
	if s.listen != "127.0.0.1:9999" {
		t.Errorf("listen = %q; want 127.0.0.1:9999", s.listen)
	}
}

// TestStartReturnsErrorOnBadAddress confirms Start surfaces a bind
// error when the listen address is invalid.
func TestStartReturnsErrorOnBadAddress(t *testing.T) {
	s := CreateServer()
	s.SetListen("invalid-host:99999")

	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Start should fail for invalid listen address")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return within 3s")
	}
}

func TestShutdownWhenServerNotStarted(t *testing.T) {
	s := CreateServer()
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown on non-started server: %v", err)
	}
}

func TestStartAndShutdown(t *testing.T) {
	s := CreateServer()
	s.SetListen("127.0.0.1:0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Start(): %v", err)
		}
	}()

	waitFor(t, 5*time.Second, "httpServer set", func() bool {
		s.mu.Lock()
		ready := s.httpServer != nil
		s.mu.Unlock()
		return ready
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	wg.Wait()
}

// TestStartRejectsDoubleStart pins the contract that calling Start a
// second time returns an error instead of overwriting the first
// http.Server reference (which would orphan the first listener).
func TestStartRejectsDoubleStart(t *testing.T) {
	s := CreateServer()
	s.SetListen("127.0.0.1:0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("first Start(): %v", err)
		}
	}()

	waitFor(t, 5*time.Second, "first httpServer set", func() bool {
		s.mu.Lock()
		ready := s.httpServer != nil
		s.mu.Unlock()
		return ready
	})

	// Capture the first server reference so we can verify it is not
	// replaced by a subsequent double-Start attempt.
	s.mu.Lock()
	first := s.httpServer
	s.mu.Unlock()
	if first == nil {
		t.Fatal("httpServer not set after first Start")
	}

	// Second Start must return an error and must NOT replace first.
	err := s.Start()
	if err == nil {
		t.Fatal("second Start() returned nil; want error")
	}
	if !strings.Contains(err.Error(), "already started") {
		t.Errorf("second Start() error = %q; want it to contain %q", err.Error(), "already started")
	}
	s.mu.Lock()
	if s.httpServer != first {
		t.Error("second Start() replaced the first http.Server reference")
	}
	s.mu.Unlock()

	// Multiple consecutive double-Start attempts must all fail
	// consistently, not just the first one.
	for i := 0; i < 3; i++ {
		if err := s.Start(); err == nil {
			t.Errorf("attempt %d: double Start() returned nil; want error", i)
		}
	}
	s.mu.Lock()
	if s.httpServer != first {
		t.Error("repeated double Start() replaced the first http.Server reference")
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	wg.Wait()
}
