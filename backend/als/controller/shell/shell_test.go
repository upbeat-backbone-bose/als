package shell

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCheckOriginCurrentBehavior documents the present (buggy) policy so
// any tightening shows up as a deliberate test change.
//
// TODO(security): the current policy is unsafe for public deployments:
//  1. Empty Origin is accepted (lets curl / native apps in, but also
//     bypasses CSWSH defense since the browser sends Origin).
//  2. Same-host comparison trusts r.Host, which can be overridden by
//     Host-header smuggling or a misconfigured reverse proxy.
//
// These cases are kept as-is on purpose. When the policy is replaced with
// an explicit allow-list, flip the wantAllow values and the next run of
// this test will surface every regression.
func TestCheckOriginCurrentBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      string
		origin    string
		wantAllow bool
	}{
		// Empty origin (e.g. curl, native app, no Origin header at all).
		{name: "empty origin is allowed", host: "als.example.com", origin: "", wantAllow: true},

		// Same host with default port handling: u.Host is "als.example.com"
		// for both http and https.
		{name: "same host http", host: "als.example.com", origin: "http://als.example.com", wantAllow: true},
		{name: "same host https", host: "als.example.com", origin: "https://als.example.com", wantAllow: true},
		{name: "same host case-insensitive", host: "ALS.example.com", origin: "https://als.example.com", wantAllow: true},
		{name: "origin scheme dropped", host: "als.example.com", origin: "https://als.example.com", wantAllow: true},

		// Different host.
		{name: "different host rejected", host: "als.example.com", origin: "https://evil.com", wantAllow: false},
		{name: "subdomain rejected", host: "als.example.com", origin: "https://api.als.example.com", wantAllow: false},
		{name: "case-insensitive host comparison", host: "als.example.com", origin: "https://ALS.example.com", wantAllow: true},

		// Host header smuggling scenarios: the comparison trusts r.Host,
		// which an attacker can set to match Origin via a misbehaving proxy.
		{name: "host header smuggle matches origin", host: "evil.com", origin: "https://evil.com", wantAllow: true},

		// Malformed Origin URL.
		{name: "malformed origin rejected", host: "als.example.com", origin: "ht!tp://als.example.com", wantAllow: false},

		// Port differences.
		{name: "port mismatch rejected", host: "als.example.com:8080", origin: "https://als.example.com:9090", wantAllow: false},
		{name: "default port 443 vs explicit", host: "als.example.com", origin: "https://als.example.com:443", wantAllow: false},

		// File / data schemes: url.Host is the part after "//", so a
		// file:// scheme with a matching hostname matches the Host header.
		// This is intentional in the current buggy implementation -- it
		// is exactly the kind of edge case the allow-list replacement
		// must lock down.
		{name: "file scheme with matching host allowed", host: "als.example.com", origin: "file://als.example.com", wantAllow: true},
		{name: "file scheme with different host rejected", host: "als.example.com", origin: "file://evil.com", wantAllow: false},
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

func TestUpgraderUsesCheckOrigin(t *testing.T) {
	// Sanity: the upgrader is wired to the checkOrigin function so the
	// policy above actually applies during real WebSocket upgrades.
	if upgrader.CheckOrigin == nil {
		t.Fatal("upgrader.CheckOrigin must not be nil")
	}

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
// if url.Parse fails, checkOrigin must return false (never true). This
// guards against a future refactor accidentally widening the policy.
func TestCheckOriginReturnsFalseOnInvalidURL(t *testing.T) {
	t.Parallel()

	tests := []string{
		// Control characters that url.Parse rejects.
		"http://\x7f",
		// Scheme-only with control chars.
		"http://foo\x00bar",
	}

	for _, origin := range tests {
		t.Run(strings.ReplaceAll(origin, "\x00", "_NUL_"), func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = "als.example.com"
			req.Header.Set("Origin", origin)

			if checkOrigin(req) {
				t.Errorf("checkOrigin(%q) = true; want false", origin)
			}
		})
	}
}
