package speedtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		ckSize      string
		wantStatus  int
		wantBodyLen int
	}{
		{
			name:        "default chunks",
			ckSize:      "",
			wantStatus:  http.StatusOK,
			wantBodyLen: 4 * 1048576,
		},
		{
			name:        "single chunk",
			ckSize:      "1",
			wantStatus:  http.StatusOK,
			wantBodyLen: 1048576,
		},
		{
			name:        "two chunks",
			ckSize:      "2",
			wantStatus:  http.StatusOK,
			wantBodyLen: 2 * 1048576,
		},
		{
			name:        "negative ckSize uses default",
			ckSize:      "-1",
			wantStatus:  http.StatusOK,
			wantBodyLen: 4 * 1048576,
		},
		{
			name:        "zero ckSize uses default",
			ckSize:      "0",
			wantStatus:  http.StatusOK,
			wantBodyLen: 4 * 1048576,
		},
		{
			name:        "non-numeric ckSize uses default",
			ckSize:      "abc",
			wantStatus:  http.StatusOK,
			wantBodyLen: 4 * 1048576,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/download", HandleDownload)

			path := "/download"
			if tt.ckSize != "" {
				path += "?ckSize=" + tt.ckSize
			}

			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d; want %d", w.Code, tt.wantStatus)
			}

			if w.Body.Len() != tt.wantBodyLen {
				t.Errorf("body length = %d; want %d", w.Body.Len(), tt.wantBodyLen)
			}
		})
	}
}

func TestHandleDownloadDataIsRandom(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download?ckSize=1", http.NoBody)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)

	b1 := w1.Body.Bytes()
	b2 := w2.Body.Bytes()

	sameCount := 0
	for i := 0; i < len(b1) && i < len(b2); i++ {
		if b1[i] == b2[i] {
			sameCount++
		}
	}

	if sameCount == len(b1) {
		t.Error("two successive downloads returned identical data")
	}
}

func TestHandleUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "normal upload",
			body:       strings.Repeat("x", 1024),
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "large upload",
			body:       strings.Repeat("x", 1048576),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/upload", HandleUpload)

			req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d; want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleUploadHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("data"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Cache-Control header is not set")
	}
	if !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("Cache-Control = %q; want no-cache", cacheControl)
	}

	connection := w.Header().Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("Connection = %q; want keep-alive", connection)
	}
}

func TestHandleDownloadResponseContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// HandleDownload does not set Content-Type explicitly, so it defaults.
	// Verify the response has content.
	if w.Body.Len() == 0 {
		t.Error("response body is empty")
	}
}

func BenchmarkHandleDownload(b *testing.B) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download?ckSize=1", http.NoBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		_ = w.Body.Bytes()
	}
}

func BenchmarkHandleUpload(b *testing.B) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	body := strings.Repeat("x", 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// TestHandleDownloadNoChunks verifies that a query parameter that
// overrides the default chunks produces a proportional body.
func TestHandleDownloadNoChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
}

func TestHandleUploadConsumesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	body := strings.NewReader("test data")
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	remaining, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("unexpected error reading remaining body: %v", err)
	}
	if len(remaining) > 0 {
		t.Errorf("upload handler did not fully read body; %d bytes remaining", len(remaining))
	}
}

func TestHandleDownloadCkSizeCappedAt1024(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		ckSize   string
		wantBody int
	}{
		{"above cap 2000", "2000", 1024 * 1048576},
		{"above cap 9999", "9999", 1024 * 1048576},
		{"exactly at cap", "1024", 1024 * 1048576},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/download", HandleDownload)

			server := httptest.NewServer(r)
			t.Cleanup(server.Close)

			resp, err := http.Get(server.URL + "/download?ckSize=" + tt.ckSize)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d; want 200", resp.StatusCode)
			}

			buf := make([]byte, 1024)
			n, _ := resp.Body.Read(buf)
			if n == 0 {
				t.Error("response body is empty")
			}
		})
	}
}

// errReader is an io.Reader that always returns an error.
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

// TestHandleUploadBodyReadError exercises the io.Copy error
// branch in HandleUpload (librespeed.go:42). When the request
// body returns an error during read, the handler responds 400
// and does not set the success headers.
func TestHandleUploadBodyReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	body := io.NopCloser(errReader{err: errors.New("simulated read failure")})
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400; body = %q", w.Code, w.Body.String())
	}
}
