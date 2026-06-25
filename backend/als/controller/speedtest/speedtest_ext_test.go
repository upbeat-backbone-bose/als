package speedtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/config"
)

func TestHandleDownloadCkSizeMaxExceedsCap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download?ckSize=9999", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
	expectedMax := 1024 * 1048576
	if w.Body.Len() != expectedMax {
		t.Errorf("body length = %d; want %d (capped at 1024 chunks)", w.Body.Len(), expectedMax)
	}
}

func TestHandleDownloadCkSizeExactlyAtCap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/download", HandleDownload)

	req := httptest.NewRequest(http.MethodGet, "/download?ckSize=1024", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
	expectedLen := 1024 * 1048576
	if w.Body.Len() != expectedLen {
		t.Errorf("body length = %d; want %d", w.Body.Len(), expectedLen)
	}
}

func TestHandleFakeFileMultipleSizes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sizes := []string{"1KB", "1MB"}
	for _, size := range sizes {
		t.Run(size, func(t *testing.T) {
			prev := config.Config
			config.Config = &config.ALSConfig{
				SpeedtestFileList: []string{size},
			}
			t.Cleanup(func() { config.Config = prev })

			r := gin.New()
			r.GET("/file/:filename", HandleFakeFile)

			server := httptest.NewServer(r)
			t.Cleanup(server.Close)

			resp, err := http.Get(server.URL + "/file/" + size + ".test")
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d; want 200", resp.StatusCode)
			}

			n, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if n == 0 {
				t.Error("response body is empty")
			}

			cl := resp.Header.Get("Content-Length")
			if cl == "" {
				t.Error("Content-Length header is missing")
			}
			clInt, _ := strconv.ParseInt(cl, 10, 64)
			if clInt != n {
				t.Errorf("Content-Length = %d; body received = %d", clInt, n)
			}
		})
	}
}

func TestHandleFakeFileInvalidFormatReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		SpeedtestFileList: []string{"1MB"},
	}
	t.Cleanup(func() { config.Config = prev })

	r := gin.New()
	r.GET("/file/:filename", HandleFakeFile)

	tests := []string{
		"invalid.txt",
		"1MB.txt",
		"something.test",
		"test",
		"",
	}
	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/file/"+filename, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("filename=%q: status = %d; want 404", filename, w.Code)
			}
		})
	}
}

func TestSizeToBytesAdditionalBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "1024KB = 1MB", input: "1024KB", want: 1024 * 1024},
		{name: "1024MB = 1GB", input: "1024MB", want: 1024 * 1024 * 1024},
		{name: "1024GB = 1TB", input: "1024GB", want: 1024 * 1024 * 1024 * 1024},
		{name: "large KB", input: "999999KB", want: 999999 * 1024},
		{name: "large MB", input: "999999MB", want: 999999 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sizeToBytes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sizeToBytes(%q) = %d; want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sizeToBytes(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("sizeToBytes(%q) = %d; want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandleUploadEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(""))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

func TestHandleUploadLargeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/upload", HandleUpload)

	body := strings.NewReader(strings.Repeat("data", 100000))
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}
