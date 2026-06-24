package speedtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/config"
)

// TestHandleFakeFileStreamsRandomBytes verifies the full handler path
// that was previously broken by an unconditional WaitQueue call: the
// request hangs forever because no queue handler is registered for
// the fakefile endpoint. This test pins the regression.
//
// gin.Stream() requires http.CloseNotifier, which httptest's
// ResponseRecorder does not implement. We run the handler on a real
// httptest.NewServer so the full stream path works.
func TestHandleFakeFileStreamsRandomBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		SpeedtestFileList: []string{"1MB"},
	}
	t.Cleanup(func() { config.Config = prev })

	r := gin.New()
	r.GET("/file/:filename", HandleFakeFile)

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	done := make(chan struct{})
	var (
		status int
		ctype  string
		clen   string
	)
	go func() {
		defer close(done)
		resp, err := http.Get(server.URL + "/file/1MB.test")
		if err != nil {
			t.Errorf("GET failed: %v", err)
			return
		}
		// Read and discard the body so the handler can complete.
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		_ = resp.Body.Close()
		status = resp.StatusCode
		ctype = resp.Header.Get("Content-Type")
		clen = resp.Header.Get("Content-Length")
	}()

	// The handler must complete -- previously it hung forever.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("HandleFakeFile did not return within 10s -- regression: stuck on WaitQueue")
	}

	if status != http.StatusOK {
		t.Errorf("status = %d; want 200", status)
	}
	if ctype != "application/octet-stream" {
		t.Errorf("Content-Type = %q; want application/octet-stream", ctype)
	}
	if clen != "1048576" {
		t.Errorf("Content-Length = %q; want 1048576", clen)
	}
}

// TestHandleFakeFileInvalidFilename covers the 404 path before any
// state was touched.
func TestHandleFakeFileInvalidFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{SpeedtestFileList: []string{"1MB"}}
	t.Cleanup(func() { config.Config = prev })

	r := gin.New()
	r.GET("/file/:filename", HandleFakeFile)

	req := httptest.NewRequest(http.MethodGet, "/file/not-a-fake.txt", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "404") {
		t.Errorf("body = %q; want '404'", w.Body.String())
	}
}

// TestHandleFakeFileNotInAllowlist covers the case where the filename
// matches the regex but is not in config.Config.SpeedtestFileList.
func TestHandleFakeFileNotInAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		SpeedtestFileList: []string{"1MB"},
	}
	t.Cleanup(func() { config.Config = prev })

	r := gin.New()
	r.GET("/file/:filename", HandleFakeFile)

	req := httptest.NewRequest(http.MethodGet, "/file/100GB.test", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}
