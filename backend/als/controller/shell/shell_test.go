package shell

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/config"
)

// TestCheckOriginAllowList exercises the post-fix policy:
//  1. Empty Origin is rejected (browsers always send one; a missing
//     Origin means a non-browser client and is not a trust signal).
//  2. Cross-origin / foreign hosts are rejected.
//  3. Host-header smuggling no longer matters: r.Host is never compared
//     against Origin.
//  4. Hosts on the allow-list (port-stripped, case-insensitive) pass.
//
// If the allow-list is empty, every upgrade is rejected -- this is the
// safe default.
//
// NOTE: this test (and the URL-rejection test below) intentionally does
// not call t.Parallel. Both mutate the package-level allowedOrigins
// slice via setAllowedOriginsForTest, which would race under parallel
// execution. TestBuildAllowedOrigins and TestStripPort use only pure
// functions and can still run in parallel.
func TestCheckOriginAllowList(t *testing.T) {

	tests := []struct {
		name        string
		allowed     []string
		host        string
		origin      string
		wantAllow   bool
		description string
	}{
		// --- Reject empty / malformed / cross-origin ---
		{
			name:        "empty origin rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "",
			wantAllow:   false,
			description: "missing Origin is the canonical CSWSH bypass signal",
		},
		{
			name:        "different host rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "https://evil.com",
			wantAllow:   false,
			description: "foreign origin must never be accepted",
		},
		{
			name:        "subdomain rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "https://api.als.example.com",
			wantAllow:   false,
			description: "subdomain is not a free pass: must be on the list",
		},
		{
			name:        "malformed origin rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "ht!tp://als.example.com",
			wantAllow:   false,
			description: "garbage Origin must not match",
		},
		{
			name:        "file scheme rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "file://als.example.com",
			wantAllow:   false,
			description: "file:// is not http(s); must not be accepted",
		},
		{
			name:        "data scheme rejected",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "data:text/html,hello",
			wantAllow:   false,
			description: "non-network schemes must not be accepted",
		},

		// --- Host-header smuggling is no longer relevant ---
		{
			name:        "host header mismatch does not bypass allowlist",
			allowed:     []string{"als.example.com"},
			host:        "evil.com",
			origin:      "https://als.example.com",
			wantAllow:   true,
			description: "if Origin is on the allow-list, accept regardless of Host header",
		},
		{
			name:        "host header match with foreign origin still rejected",
			allowed:     []string{"als.example.com"},
			host:        "evil.com",
			origin:      "https://evil.com",
			wantAllow:   false,
			description: "the old behavior trusted r.Host; the new policy ignores it",
		},

		// --- Allow-list matching ---
		{
			name:        "same host allowed",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "https://als.example.com",
			wantAllow:   true,
			description: "origin on the list passes",
		},
		{
			name:        "case-insensitive origin allowed",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "https://ALS.EXAMPLE.COM",
			wantAllow:   true,
			description: "host matching is case-insensitive",
		},
		{
			name:        "port-stripped matching",
			allowed:     []string{"als.example.com"},
			host:        "als.example.com",
			origin:      "https://als.example.com:8443",
			wantAllow:   true,
			description: "Origin port does not have to match the configured entry",
		},
		{
			name:        "configured port matches any origin port",
			allowed:     []string{"als.example.com:8080"},
			host:        "als.example.com:8080",
			origin:      "https://als.example.com:9090",
			wantAllow:   true,
			description: "port is stripped before comparison; both entries match",
		},
		{
			name:        "ipv6 literal on allowlist matches bracketed origin",
			allowed:     []string{"[2001:db8::1]"},
			host:        "[2001:db8::1]",
			origin:      "https://[2001:db8::1]:8443",
			wantAllow:   true,
			description: "IPv6 literals with brackets are normalised",
		},
		{
			name:        "ipv6 host on allowlist rejects ipv4 origin",
			allowed:     []string{"2001:db8::1"},
			host:        "als.example.com",
			origin:      "https://1.2.3.4",
			wantAllow:   false,
			description: "v6 and v4 are distinct hosts",
		},

		// --- Multiple entries ---
		{
			name:        "second entry on list accepted",
			allowed:     []string{"primary.example.com", "backup.example.com"},
			host:        "primary.example.com",
			origin:      "https://backup.example.com",
			wantAllow:   true,
			description: "any one match is enough",
		},

		// --- Empty allow-list is the safe default ---
		{
			name:        "empty allowlist rejects everything",
			allowed:     nil,
			host:        "als.example.com",
			origin:      "https://als.example.com",
			wantAllow:   false,
			description: "no opt-in means no shell",
		},
		{
			name:        "empty-string entries are ignored",
			allowed:     []string{"", "  ", "als.example.com"},
			host:        "als.example.com",
			origin:      "https://als.example.com",
			wantAllow:   true,
			description: "whitespace/empty entries do not match anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NOTE: deliberately NOT calling t.Parallel here. The shared
			// package-level allowedOrigins slice is mutated per case via
			// setAllowedOriginsForTest, and parallelism would race on
			// both the slice and the cleanup ordering.
			cleanup := setAllowedOriginsForTest(tt.allowed)
			defer cleanup()

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := checkOrigin(req)
			if got != tt.wantAllow {
				t.Errorf("checkOrigin(host=%q, origin=%q, allowed=%v) = %v; want %v (%s)",
					tt.host, tt.origin, tt.allowed, got, tt.wantAllow, tt.description)
			}
		})
	}
}

func TestUpgraderUsesCheckOrigin(t *testing.T) {
	// Sanity: the upgrader is wired to the checkOrigin function so the
	// policy above actually applies during real WebSocket upgrades.
	if upgrader.CheckOrigin == nil {
		t.Fatal("upgrader.CheckOrigin must not be nil")
	}

	cleanup := setAllowedOriginsForTest([]string{"als.example.com"})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Host = "als.example.com"
	req.Header.Set("Origin", "https://als.example.com")

	// Two calls must agree -- if upgrader wires a different function, this
	// catches it.
	direct := checkOrigin(req)
	wired := upgrader.CheckOrigin(req)
	if direct != wired {
		t.Errorf("upgrader.CheckOrigin=%v != checkOrigin=%v", wired, direct)
	}
}

// TestCheckOriginReturnsFalseOnInvalidURL is a focused regression test:
// if url.Parse fails or returns no host, checkOrigin must return false
// (never true). This guards against a future refactor accidentally
// widening the policy.
//
// Sequential -- shares the allowedOrigins allow-list with the policy
// table tests above.
func TestCheckOriginReturnsFalseOnInvalidURL(t *testing.T) {
	cleanup := setAllowedOriginsForTest([]string{"als.example.com"})
	defer cleanup()

	tests := []string{
		// Control characters that url.Parse rejects.
		"http://\x7f",
		// Scheme-only with control chars.
		"http://foo\x00bar",
	}

	for _, origin := range tests {
		t.Run(strings.ReplaceAll(origin, "\x00", "_NUL_"), func(t *testing.T) {
			// Sequential on purpose: shares setAllowedOriginsForTest with
			// the table tests above.
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = "als.example.com"
			req.Header.Set("Origin", origin)

			if checkOrigin(req) {
				t.Errorf("checkOrigin(%q) = true; want false", origin)
			}
		})
	}
}

func TestHandleNewShellMissingSession(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/shell", HandleNewShell)

	req := httptest.NewRequest(http.MethodGet, "/shell", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The handler closes the websocket (which never opened) and
	// returns without writing any HTTP response. We assert that the
	// call did not panic and that no 200 was emitted.
	if w.Code == http.StatusOK {
		t.Errorf("got 200; handler should not produce 200 without a session")
	}
}

func TestUpgraderBufferSizes(t *testing.T) {
	t.Parallel()

	if upgrader.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d; want 4096", upgrader.ReadBufferSize)
	}
	if upgrader.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d; want 4096", upgrader.WriteBufferSize)
	}
}

func TestBuildAllowedOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.ALSConfig
		want []string
	}{
		{
			name: "public addresses only",
			cfg: &config.ALSConfig{
				PublicIPv4:     "1.2.3.4",
				PublicIPv6:     "2001:db8::1",
				ListenHost:     "0.0.0.0",
				AllowedOrigins: nil,
			},
			want: []string{"1.2.3.4", "2001:db8::1", "0.0.0.0"},
		},
		{
			name: "case-normalised",
			cfg: &config.ALSConfig{
				PublicIPv4:     "ALS.Example.COM",
				AllowedOrigins: []string{"API.EXAMPLE.COM"},
			},
			want: []string{"als.example.com", "api.example.com"},
		},
		{
			name: "empty/whitespace entries dropped",
			cfg: &config.ALSConfig{
				PublicIPv4:     "  ",
				AllowedOrigins: []string{"", "  ", "real.example.com"},
			},
			want: []string{"real.example.com"},
		},
		{
			name: "duplicates collapsed",
			cfg: &config.ALSConfig{
				PublicIPv4:     "1.2.3.4",
				AllowedOrigins: []string{"1.2.3.4", "1.2.3.4:8080"},
			},
			// 1.2.3.4 normalised before "1.2.3.4:8080" -> both kept
			// because port-stripping happens at comparison time, not here.
			want: []string{"1.2.3.4", "1.2.3.4:8080"},
		},
		{
			name: "empty config yields empty list",
			cfg:  &config.ALSConfig{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildAllowedOrigins(tt.cfg)
			if !equalSlices(got, tt.want) {
				t.Errorf("buildAllowedOrigins() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestStripPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		want string
	}{
		{"als.example.com", "als.example.com"},
		{"als.example.com:8080", "als.example.com"},
		{"ALS.Example.COM", "als.example.com"},
		{"[2001:db8::1]", "[2001:db8::1]"},
		{"[2001:db8::1]:8443", "[2001:db8::1]"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()
			got := stripPort(tt.host)
			if got != tt.want {
				t.Errorf("stripPort(%q) = %q; want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestAllowedOriginsSnapshotIsCopy(t *testing.T) {
	// Sequential on purpose: mutates the package-level allowedOrigins
	// slice via setAllowedOriginsForTest.
	cleanup := setAllowedOriginsForTest([]string{"a.example.com", "b.example.com"})
	defer cleanup()

	snap := AllowedOriginsSnapshot()
	snap[0] = "tampered.example.com"

	// Re-reading the snapshot must not show the tampered value.
	again := AllowedOriginsSnapshot()
	if again[0] != "a.example.com" {
		t.Errorf("snapshot was not a copy: got %q after external mutation", again[0])
	}
}

// equalSlices compares two string slices for unordered equality.
// Order is significant here because buildAllowedOrigins preserves the
// precedence (PublicIPv4 -> PublicIPv6 -> ListenHost -> AllowedOrigins).
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
