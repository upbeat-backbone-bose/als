package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWebConfigIPLookupSkippedWhenBothSet verifies that when
// both PublicIPv4 and PublicIPv6 are pre-set via env, LoadWebConfig
// does not spawn the IP-lookup goroutine (which would race with
// test cleanup because it makes real DNS/HTTP calls). We set
// PUBLIC_IPV4 + PUBLIC_IPV6 + LOCATION env vars, then install a
// transport that fails any outbound request as a safety net: if a
// future regression starts the goroutine again, the test fails
// fast via the t.Errorf in the handler.
//
// The iperf3 feature flag is gated on exec.LookPath("iperf3").
// If the developer's PATH happens to include iperf3 (e.g. they
// installed it for benchmarking), the flag would otherwise be set
// to true and the "FeatureIperf3 = false" assertion below would
// fail for the wrong reason -- masking the actual contract we
// want to verify. We isolate PATH to an empty temp dir for the
// duration of this test so LookPath fails regardless of host
// environment. This makes the test environment-independent.
func TestLoadWebConfigIPLookupSkippedWhenBothSet(t *testing.T) {
	prev := Config
	Config = &ALSConfig{}
	t.Cleanup(func() { Config = prev })
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	// Isolate PATH to a fresh empty dir so exec.LookPath("iperf3")
	// returns an error on every host. The dir is empty by
	// construction; we never write to it.
	t.Setenv("PATH", t.TempDir())

	withEnv(t, map[string]string{
		"PUBLIC_IPV4": "1.2.3.4",
		"PUBLIC_IPV6": "::1",
		"LOCATION":    "Earth",
	})

	// Safety net: any outbound request fails loudly. If a future
	// regression starts the IP-lookup goroutine, the test fails
	// fast instead of silently leaking DNS/HTTP traffic.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected outbound HTTP request to %s", r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	installTestTransport(t, server.URL)

	LoadWebConfig()

	if Config.PublicIPv4 != "1.2.3.4" {
		t.Errorf("PublicIPv4 = %q; want unchanged", Config.PublicIPv4)
	}
	if Config.PublicIPv6 != "::1" {
		t.Errorf("PublicIPv6 = %q; want unchanged", Config.PublicIPv6)
	}
	if Config.Location != "Earth" {
		t.Errorf("Location = %q; want unchanged", Config.Location)
	}

	// iperf3 is not on PATH (empty safety-net dir): the
	// LoadWebConfig must detect this and set FeatureIperf3=false.
	if Config.FeatureIperf3 {
		t.Error("FeatureIperf3 = true; want false (iperf3 is not on PATH)")
	}
}

// TestLoadWebConfigIperf3OnPathPreemptsOverride covers the path
// where iperf3 IS on PATH. We drop a fake iperf3 binary into a
// temp dir and prepend it to PATH, then assert FeatureIperf3
// remains true after LoadWebConfig.
//
// This pins the inverse of the "not on PATH" path: a regression
// that unconditionally clears FeatureIperf3 would be caught.
func TestLoadWebConfigIperf3OnPathPreemptsOverride(t *testing.T) {
	prev := Config
	Config = &ALSConfig{}
	t.Cleanup(func() { Config = prev })
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	withEnv(t, map[string]string{
		"PUBLIC_IPV4": "1.2.3.4",
		"PUBLIC_IPV6": "::1",
	})

	// Drop a fake iperf3 onto PATH.
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "iperf3")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake iperf3: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Same safety net for the IP-lookup goroutine.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected outbound HTTP request to %s", r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	installTestTransport(t, server.URL)

	// Pre-set FeatureIperf3=true so we can verify the path
	// where iperf3 IS available does not clear it.
	Config.FeatureIperf3 = true

	LoadWebConfig()

	if !Config.FeatureIperf3 {
		t.Error("FeatureIperf3 = false; want true (iperf3 is on PATH)")
	}
}

func TestLoadSponsorMessageEmpty(t *testing.T) {
	// Empty SponsorMessage: function must return immediately without
	// touching anything.
	withConfig(t, &ALSConfig{SponsorMessage: ""})
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

	withConfig(t, &ALSConfig{SponsorMessage: path})
	LoadSponsorMessage()

	if Config.SponsorMessage != content {
		t.Errorf("SponsorMessage = %q; want %q", Config.SponsorMessage, content)
	}
}

func TestLoadSponsorMessageFromLocalFileMissing(t *testing.T) {
	// Path that does not exist: os.Stat errors, http.Get errors,
	// SponsorMessage must remain unchanged.

	withConfig(t, &ALSConfig{SponsorMessage: "/nonexistent/path/" + fmt.Sprint(t) + "/sponsor"})
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

	withConfig(t, &ALSConfig{SponsorMessage: server.URL})
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
	withConfig(t, &ALSConfig{SponsorMessage: original})
	LoadSponsorMessage()

	// Non-2xx: must keep original URL (no replacement with body).
	if Config.SponsorMessage != original {
		t.Errorf("SponsorMessage = %q; want %q", Config.SponsorMessage, original)
	}
}

func TestLoadSponsorMessageFromHTTPUnreachable(t *testing.T) {
	// Unroutable address: http.Get must fail, SponsorMessage keeps the
	// original value.

	withConfig(t, &ALSConfig{SponsorMessage: "http://127.0.0.1:1/sponsor"})
	LoadSponsorMessage()

	if Config.SponsorMessage != "http://127.0.0.1:1/sponsor" {
		t.Errorf("SponsorMessage changed despite unreachable URL")
	}
}
