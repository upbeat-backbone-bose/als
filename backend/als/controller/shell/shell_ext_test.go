package shell

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpgraderBufferSizes(t *testing.T) {
	t.Parallel()

	if upgrader.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d; want 4096", upgrader.ReadBufferSize)
	}
	if upgrader.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d; want 4096", upgrader.WriteBufferSize)
	}
}

func TestCheckOriginLocalhost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      string
		origin    string
		wantAllow bool
	}{
		{
			name:      "localhost same",
			host:      "localhost:8080",
			origin:    "http://localhost:8080",
			wantAllow: true,
		},
		{
			name:      "127.0.0.1",
			host:      "127.0.0.1:8080",
			origin:    "http://127.0.0.1:8080",
			wantAllow: true,
		},
		{
			name:      "localhost vs 127.0.0.1 rejected",
			host:      "localhost:8080",
			origin:    "http://127.0.0.1:8080",
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := checkOrigin(req)
			if got != tt.wantAllow {
				t.Errorf("checkOrigin(host=%q, origin=%q) = %v; want %v",
					tt.host, tt.origin, got, tt.wantAllow)
			}
		})
	}
}

func TestCheckOriginHostWithPort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Host = "example.com:8080"
	req.Header.Set("Origin", "http://example.com:8080")

	if !checkOrigin(req) {
		t.Error("checkOrigin should allow when host:port matches")
	}
}

func TestCheckOriginNoOriginHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Host = "example.com"

	if !checkOrigin(req) {
		t.Error("checkOrigin should allow when no Origin header")
	}
}
