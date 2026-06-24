package session

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/samlm0/als/v2/config"
)

func TestBuildClientConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.ALSConfig
		ip   string
		want ClientConfig
	}{
		{
			name: "zero value config",
			cfg:  &config.ALSConfig{},
			ip:   "1.2.3.4",
			want: ClientConfig{ClientIP: "1.2.3.4"},
		},
		{
			name: "all client-facing fields populated",
			cfg: &config.ALSConfig{
				Location:             "Earth",
				PublicIPv4:           "1.2.3.4",
				PublicIPv6:           "::1",
				SpeedtestFileList:    []string{"1MB", "10MB"},
				SponsorMessage:       "hi",
				FeaturePing:          true,
				FeatureShell:         true,
				FeatureLibrespeed:    true,
				FeatureFileSpeedtest: true,
			},
			ip: "9.9.9.9",
			want: ClientConfig{
				ClientIP:             "9.9.9.9",
				Location:             "Earth",
				PublicIPv4:           "1.2.3.4",
				PublicIPv6:           "::1",
				SpeedtestFileList:    []string{"1MB", "10MB"},
				SponsorMessage:       "hi",
				FeaturePing:          true,
				FeatureShell:         true,
				FeatureLibrespeed:    true,
				FeatureFileSpeedtest: true,
			},
		},
		{
			name: "internal fields are not propagated",
			cfg: &config.ALSConfig{
				ListenHost:      "127.0.0.1",
				ListenPort:      "8080",
				Iperf3StartPort: 30000,
				Iperf3EndPort:   31000,
			},
			ip:   "9.9.9.9",
			want: ClientConfig{ClientIP: "9.9.9.9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildClientConfig(tt.cfg, tt.ip)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildClientConfig() = %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestClientConfigJSONOmitsInternalFields(t *testing.T) {
	forbidden := []string{
		"listen_host",
		"listen_port",
		"iperf3_start_port",
		"iperf3_end_port",
	}

	cfg := &config.ALSConfig{
		ListenHost:        "127.0.0.1",
		ListenPort:        "8080",
		Iperf3StartPort:   30000,
		Iperf3EndPort:     31000,
		SpeedtestFileList: []string{"1MB"},
	}
	got := buildClientConfig(cfg, "1.2.3.4")

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(b)

	for _, field := range forbidden {
		if strings.Contains(s, field) {
			t.Errorf("internal field %q leaked into client JSON: %s", field, s)
		}
	}
}

func TestClientConfigJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   ClientConfig
	}{
		{
			name: "empty",
			in:   ClientConfig{},
		},
		{
			name: "all features on",
			in: ClientConfig{
				ClientIP:               "9.9.9.9",
				Location:               "Earth",
				PublicIPv4:             "1.2.3.4",
				PublicIPv6:             "::1",
				SpeedtestFileList:      []string{"1MB", "10MB"},
				SponsorMessage:         "sponsor",
				FeaturePing:            true,
				FeatureShell:           true,
				FeatureLibrespeed:      true,
				FeatureFileSpeedtest:   true,
				FeatureSpeedtestDotNet: true,
				FeatureIperf3:          true,
				FeatureMTR:             true,
				FeatureTraceroute:      true,
				FeatureIfaceTraffic:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var got ClientConfig
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if !reflect.DeepEqual(got, tt.in) {
				t.Errorf("round-trip mismatch: got %+v; want %+v", got, tt.in)
			}
		})
	}
}
