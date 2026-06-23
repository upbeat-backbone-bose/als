package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

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

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
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