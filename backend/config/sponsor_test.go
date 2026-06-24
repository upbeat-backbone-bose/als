package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWebConfigSkippedDueToGoroutine(t *testing.T) {
	// LoadWebConfig spawns a goroutine that calls updatePublicIP and
	// updateLocation when both PublicIPv4 and PublicIPv6 are empty.
	// Both functions depend on real DNS/HTTP and would race with
	// test cleanup. Until LoadWebConfig is refactored to expose an
	// injection point for the IP-lookup client, the only testable
	// surface -- the iperf3 binary presence check -- cannot be
	// exercised in isolation.
	t.Skip("LoadWebConfig spawns an IP-lookup goroutine that races with cleanup; needs refactor")
}

func TestLoadSponsorMessageEmpty(t *testing.T) {
	// Empty SponsorMessage: function must return immediately without
	// touching anything.
	Config = &ALSConfig{SponsorMessage: ""}
	LoadSponsorMessage()
	if Config.SponsorMessage != "" {
		t.Errorf("SponsorMessage = %q; want empty", Config.SponsorMessage)
	}
}

func TestLoadSponsorMessageFromLocalFile(t *testing.T) {

	dir := t.TempDir()
	path := filepath.Join(dir, "sponsor.md")
	content := "# Hello from file"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	Config = &ALSConfig{SponsorMessage: path}
	LoadSponsorMessage()

	if Config.SponsorMessage != content {
		t.Errorf("SponsorMessage = %q; want %q", Config.SponsorMessage, content)
	}
}

func TestLoadSponsorMessageFromLocalFileMissing(t *testing.T) {
	// Path that does not exist: os.Stat errors, http.Get errors,
	// SponsorMessage must remain unchanged.

	Config = &ALSConfig{SponsorMessage: "/nonexistent/path/" + fmt.Sprint(t) + "/sponsor"}
	LoadSponsorMessage()

	if Config.SponsorMessage == "" {
		t.Errorf("SponsorMessage should remain unchanged on failure")
	}
}

func TestLoadSponsorMessageFromHTTPSuccess(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "# Sponsor from URL")
	}))
	t.Cleanup(server.Close)

	Config = &ALSConfig{SponsorMessage: server.URL}
	LoadSponsorMessage()

	if Config.SponsorMessage != "# Sponsor from URL" {
		t.Errorf("SponsorMessage = %q; want %q", Config.SponsorMessage, "# Sponsor from URL")
	}
}

func TestLoadSponsorMessageFromHTTPNotOK(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	original := server.URL
	Config = &ALSConfig{SponsorMessage: original}
	LoadSponsorMessage()

	// Non-2xx: must keep original URL (no replacement with body).
	if Config.SponsorMessage != original {
		t.Errorf("SponsorMessage = %q; want %q", Config.SponsorMessage, original)
	}
}

func TestLoadSponsorMessageFromHTTPUnreachable(t *testing.T) {
	// Unroutable address: http.Get must fail, SponsorMessage keeps the
	// original value.

	Config = &ALSConfig{SponsorMessage: "http://127.0.0.1:1/sponsor"}
	LoadSponsorMessage()

	if Config.SponsorMessage != "http://127.0.0.1:1/sponsor" {
		t.Errorf("SponsorMessage changed despite unreachable URL")
	}
}
