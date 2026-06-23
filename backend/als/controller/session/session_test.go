package session

import (
	"encoding/json"
	"testing"

	"github.com/samlm0/als/v2/config"
)

func TestBuildClientConfigOnlyExposesExpectedFields(t *testing.T) {
	cfg := &config.ALSConfig{
		ListenHost:      "127.0.0.1",
		ListenPort:      "8080",
		Location:        "Earth",
		PublicIPv4:      "1.2.3.4",
		PublicIPv6:      "::1",
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,
		SpeedtestFileList: []string{"1MB", "10MB"},
		SponsorMessage:  "hi",
		FeaturePing:     true,
		FeatureShell:    true,
	}

	got := buildClientConfig(cfg, "9.9.9.9")

	if got.ClientIP != "9.9.9.9" {
		t.Errorf("ClientIP = %q, want %q", got.ClientIP, "9.9.9.9")
	}
	if got.Location != "Earth" {
		t.Errorf("Location = %q, want %q", got.Location, "Earth")
	}
	if got.PublicIPv4 != "1.2.3.4" {
		t.Errorf("PublicIPv4 = %q, want %q", got.PublicIPv4, "1.2.3.4")
	}
	if got.PublicIPv6 != "::1" {
		t.Errorf("PublicIPv6 = %q, want %q", got.PublicIPv6, "::1")
	}
	if len(got.SpeedtestFileList) != 2 || got.SpeedtestFileList[0] != "1MB" {
		t.Errorf("SpeedtestFileList = %v, want [1MB 10MB]", got.SpeedtestFileList)
	}
	if !got.FeaturePing || !got.FeatureShell {
		t.Errorf("feature flags not propagated: %+v", got)
	}
}

func TestBuildClientConfigJSONOmitsInternalFields(t *testing.T) {
	cfg := &config.ALSConfig{
		ListenHost:       "127.0.0.1",
		ListenPort:       "8080",
		Iperf3StartPort:  30000,
		Iperf3EndPort:    31000,
		SpeedtestFileList: []string{"1MB"},
	}
	got := buildClientConfig(cfg, "9.9.9.9")

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(b)

	for _, leaked := range []string{"listen_host", "listen_port", "iperf3_start_port", "iperf3_end_port"} {
		if contains(s, leaked) {
			t.Errorf("internal field %q leaked into client config JSON: %s", leaked, s)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}