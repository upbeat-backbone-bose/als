package config

import (
	"testing"
)

func TestGetDefaultConfigAllFeaturesTrue(t *testing.T) {
	got := GetDefaultConfig()

	wantTrue := map[string]bool{
		"FeaturePing":            got.FeaturePing,
		"FeatureShell":           got.FeatureShell,
		"FeatureLibrespeed":      got.FeatureLibrespeed,
		"FeatureFileSpeedtest":   got.FeatureFileSpeedtest,
		"FeatureSpeedtestDotNet": got.FeatureSpeedtestDotNet,
		"FeatureIperf3":          got.FeatureIperf3,
		"FeatureMTR":             got.FeatureMTR,
		"FeatureTraceroute":      got.FeatureTraceroute,
		"FeatureIfaceTraffic":    got.FeatureIfaceTraffic,
	}

	for name, val := range wantTrue {
		if !val {
			t.Errorf("%s = %v; want true", name, val)
		}
	}
}

func TestGetDefaultConfigDefaults(t *testing.T) {
	got := GetDefaultConfig()

	if got.ListenHost != "0.0.0.0" {
		t.Errorf("ListenHost = %q", got.ListenHost)
	}
	if got.ListenPort != "80" {
		t.Errorf("ListenPort = %q", got.ListenPort)
	}
	if got.Iperf3StartPort != 30000 {
		t.Errorf("Iperf3StartPort = %d", got.Iperf3StartPort)
	}
	if got.Iperf3EndPort != 31000 {
		t.Errorf("Iperf3EndPort = %d", got.Iperf3EndPort)
	}
	if got.Location != "" {
		t.Errorf("Location = %q; want empty", got.Location)
	}
	if got.PublicIPv4 != "" {
		t.Errorf("PublicIPv4 = %q; want empty", got.PublicIPv4)
	}
	if got.PublicIPv6 != "" {
		t.Errorf("PublicIPv6 = %q; want empty", got.PublicIPv6)
	}
}

func TestGetDefaultConfigSpeedtestFileListValues(t *testing.T) {
	got := GetDefaultConfig()
	want := []string{"1MB", "10MB", "100MB", "1GB", "100GB"}
	if len(got.SpeedtestFileList) != len(want) {
		t.Fatalf("len = %d; want %d", len(got.SpeedtestFileList), len(want))
	}
	for i, v := range want {
		if got.SpeedtestFileList[i] != v {
			t.Errorf("SpeedtestFileList[%d] = %q; want %q", i, got.SpeedtestFileList[i], v)
		}
	}
}

func TestLoadFromEnvSpeedtestFileListEmpty(t *testing.T) {
	withEnv(t, map[string]string{"SPEEDTEST_FILE_LIST": ""})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	if len(Config.SpeedtestFileList) == 0 {
		t.Error("empty env var must not overwrite default SpeedtestFileList")
	}
}

func TestLoadFromEnvSpeedtestFileListSingleItem(t *testing.T) {
	withEnv(t, map[string]string{"SPEEDTEST_FILE_LIST": "1GB"})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	if len(Config.SpeedtestFileList) != 1 {
		t.Fatalf("len = %d; want 1", len(Config.SpeedtestFileList))
	}
	if Config.SpeedtestFileList[0] != "1GB" {
		t.Errorf("SpeedtestFileList[0] = %q", Config.SpeedtestFileList[0])
	}
}

func TestLoadFromEnvLogsWhenExternalCall(t *testing.T) {
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = false
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()
}

func TestLoadPreservesEnvOverrides(t *testing.T) {
	withEnv(t, map[string]string{
		"LISTEN_IP":   "127.0.0.1",
		"HTTP_PORT":   "9090",
		"LOCATION":    "TestCity",
		"PUBLIC_IPV4": "9.9.9.9",
		"PUBLIC_IPV6": "fe80::1",
	})
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	Load()

	if Config.ListenHost != "127.0.0.1" {
		t.Errorf("ListenHost = %q", Config.ListenHost)
	}
	if Config.ListenPort != "9090" {
		t.Errorf("ListenPort = %q", Config.ListenPort)
	}
	if Config.Location != "TestCity" {
		t.Errorf("Location = %q", Config.Location)
	}
	if Config.PublicIPv4 != "9.9.9.9" {
		t.Errorf("PublicIPv4 = %q", Config.PublicIPv4)
	}
	if Config.PublicIPv6 != "fe80::1" {
		t.Errorf("PublicIPv6 = %q", Config.PublicIPv6)
	}
}

func TestLoadFromEnvBoolMultipleValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"exact true", "true", true},
		{"exact false", "false", false},
		{"empty string", "", false},
		{"random text", "yes", false},
		{"uppercase TRUE", "TRUE", false},
		{"mixed True", "True", false},
		{"numeric 0", "0", false},
		{"numeric 1", "1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withEnv(t, map[string]string{"UTILITIES_PING": tt.value})
			Config = &ALSConfig{}
			prevInternal := IsInternalCall
			IsInternalCall = true
			t.Cleanup(func() { IsInternalCall = prevInternal })

			LoadFromEnv()

			if Config.FeaturePing != tt.want {
				t.Errorf("FeaturePing = %v; want %v", Config.FeaturePing, tt.want)
			}
		})
	}
}

func TestLoadFromEnvIntValidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{"positive", "50000", 50000},
		{"zero", "0", 0},
		{"large", "65535", 65535},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withEnv(t, map[string]string{"UTILITIES_IPERF3_PORT_MIN": tt.value})
			Config = GetDefaultConfig()
			prevInternal := IsInternalCall
			IsInternalCall = true
			t.Cleanup(func() { IsInternalCall = prevInternal })

			LoadFromEnv()

			if Config.Iperf3StartPort != tt.want {
				t.Errorf("Iperf3StartPort = %d; want %d", Config.Iperf3StartPort, tt.want)
			}
		})
	}
}

func TestLoadPreservesDefaultWhenNoEnv(t *testing.T) {
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	Load()

	if Config == nil {
		t.Fatal("Config is nil after Load()")
	}
	if Config.ListenHost != "0.0.0.0" {
		t.Errorf("ListenHost = %q; want 0.0.0.0", Config.ListenHost)
	}
	def := GetDefaultConfig()
	if Config.Iperf3StartPort != def.Iperf3StartPort {
		t.Errorf("Iperf3StartPort = %d; want %d", Config.Iperf3StartPort, def.Iperf3StartPort)
	}
}
