package config

import (
	"reflect"
	"testing"
)

// withEnv sets env vars for the duration of t, restoring originals after.
// Returns a cleanup function for use with t.Cleanup.
func withEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	originals := make(map[string]string, len(vars))
	missing := make(map[string]struct{}, len(vars))
	for k := range vars {
		if v, ok := lookup(k); ok {
			originals[k] = v
		} else {
			missing[k] = struct{}{}
		}
	}
	for k, v := range vars {
		setenv(k, v)
	}
	t.Cleanup(func() {
		for k := range vars {
			if _, was := missing[k]; was {
				unsetenv(k)
			} else {
				setenv(k, originals[k])
			}
		}
	})
}

func TestGetDefaultConfig(t *testing.T) {

	got := GetDefaultConfig()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"ListenHost", got.ListenHost, "0.0.0.0"},
		{"ListenPort", got.ListenPort, "80"},
		{"Location", got.Location, ""},
		{"Iperf3StartPort", got.Iperf3StartPort, 30000},
		{"Iperf3EndPort", got.Iperf3EndPort, 31000},
		{"SpeedtestFileList", got.SpeedtestFileList, []string{"1MB", "10MB", "100MB", "1GB", "100GB"}},
		{"FeaturePing", got.FeaturePing, true},
		{"FeatureShell", got.FeatureShell, true},
		{"FeatureLibrespeed", got.FeatureLibrespeed, true},
		{"FeatureFileSpeedtest", got.FeatureFileSpeedtest, true},
		{"FeatureSpeedtestDotNet", got.FeatureSpeedtestDotNet, true},
		{"FeatureIperf3", got.FeatureIperf3, true},
		{"FeatureMTR", got.FeatureMTR, true},
		{"FeatureTraceroute", got.FeatureTraceroute, true},
		{"FeatureIfaceTraffic", got.FeatureIfaceTraffic, true},
	}
	for _, c := range checks {
		if !equalish(c.got, c.want) {
			t.Errorf("%s = %v; want %v", c.name, c.got, c.want)
		}
	}
}

// equalish compares two values, falling back to reflect.DeepEqual for
// slices and maps (which `==` rejects).
func equalish(a, b any) bool {
	switch a.(type) {
	case []string:
		return reflect.DeepEqual(a, b)
	}
	return a == b
}

func TestLoadFromEnvStringFields(t *testing.T) {
	withEnv(t, map[string]string{
		"LISTEN_IP":       "127.0.0.1",
		"HTTP_PORT":       "8080",
		"LOCATION":        "Earth",
		"PUBLIC_IPV4":     "1.2.3.4",
		"PUBLIC_IPV6":     "::1",
		"SPONSOR_MESSAGE": "hi",
	})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() {
		IsInternalCall = prevInternal
	})

	LoadFromEnv()

	if Config.ListenHost != "127.0.0.1" {
		t.Errorf("ListenHost = %q", Config.ListenHost)
	}
	if Config.ListenPort != "8080" {
		t.Errorf("ListenPort = %q", Config.ListenPort)
	}
	if Config.Location != "Earth" {
		t.Errorf("Location = %q", Config.Location)
	}
	if Config.PublicIPv4 != "1.2.3.4" {
		t.Errorf("PublicIPv4 = %q", Config.PublicIPv4)
	}
	if Config.PublicIPv6 != "::1" {
		t.Errorf("PublicIPv6 = %q", Config.PublicIPv6)
	}
	if Config.SponsorMessage != "hi" {
		t.Errorf("SponsorMessage = %q", Config.SponsorMessage)
	}
}

func TestLoadFromEnvIntFields(t *testing.T) {
	withEnv(t, map[string]string{
		"UTILITIES_IPERF3_PORT_MIN": "40000",
		"UTILITIES_IPERF3_PORT_MAX": "50000",
	})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	if Config.Iperf3StartPort != 40000 {
		t.Errorf("Iperf3StartPort = %d", Config.Iperf3StartPort)
	}
	if Config.Iperf3EndPort != 50000 {
		t.Errorf("Iperf3EndPort = %d", Config.Iperf3EndPort)
	}
}

func TestLoadFromEnvIntFieldsInvalidIgnored(t *testing.T) {
	// Invalid integer env vars must not overwrite the default; the loop
	// continues to the next entry.
	withEnv(t, map[string]string{
		"UTILITIES_IPERF3_PORT_MIN": "not-a-number",
	})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	if Config.Iperf3StartPort != 30000 {
		t.Errorf("invalid int must be ignored; got %d", Config.Iperf3StartPort)
	}
}

func TestLoadFromEnvBoolFields(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true sets true", "true", true},
		{"empty stays default (set via default)", "true", true},
		{"anything else sets false", "false", false},
		{"true is exact match only", "True", false}, // case-sensitive
		{"numeric 1 is false", "1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withEnv(t, map[string]string{"UTILITIES_PING": tt.value})
			Config = GetDefaultConfig()
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

func TestLoadFromEnvAllBoolFields(t *testing.T) {
	// Toggle every bool field via env and verify propagation.
	withEnv(t, map[string]string{
		"DISPLAY_TRAFFIC":           "true",
		"ENABLE_SPEEDTEST":          "true",
		"UTILITIES_SPEEDTESTDOTNET": "true",
		"UTILITIES_PING":            "true",
		"UTILITIES_FAKESHELL":       "true",
		"UTILITIES_IPERF3":          "true",
		"UTILITIES_MTR":             "true",
		"UTILITIES_FILESPEEDTEST":   "true",
		"UTILITIES_TRACEROUTE":      "true",
	})
	// Start from a config where everything is false.
	Config = &ALSConfig{}
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	want := []bool{
		Config.FeatureIfaceTraffic,
		Config.FeatureLibrespeed,
		Config.FeatureSpeedtestDotNet,
		Config.FeaturePing,
		Config.FeatureShell,
		Config.FeatureIperf3,
		Config.FeatureMTR,
		Config.FeatureFileSpeedtest,
		Config.FeatureTraceroute,
	}
	for i, w := range want {
		if !w {
			t.Errorf("bool field %d not set to true after LoadFromEnv", i)
		}
	}
}

func TestLoadFromEnvSpeedtestFileList(t *testing.T) {
	withEnv(t, map[string]string{
		"SPEEDTEST_FILE_LIST": "1MB 10MB 100MB",
	})
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	if len(Config.SpeedtestFileList) != 3 {
		t.Fatalf("SpeedtestFileList len = %d; want 3", len(Config.SpeedtestFileList))
	}
	if Config.SpeedtestFileList[0] != "1MB" || Config.SpeedtestFileList[2] != "100MB" {
		t.Errorf("SpeedtestFileList = %v", Config.SpeedtestFileList)
	}
}

func TestLoadFromEnvEmptyVarsKeepDefaults(t *testing.T) {
	// No env vars set: Config should equal GetDefaultConfig() after Load().
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()

	def := GetDefaultConfig()
	if Config.ListenHost != def.ListenHost {
		t.Errorf("ListenHost changed despite empty env")
	}
	if Config.Iperf3StartPort != def.Iperf3StartPort {
		t.Errorf("Iperf3StartPort changed despite empty env")
	}
	if !Config.FeaturePing {
		t.Errorf("FeaturePing became false despite empty env")
	}
}

func TestLoadFromEnvLogsWhenNotInternalCall(t *testing.T) {
	// When IsInternalCall is false the function logs -- we just verify it
	// does not panic; the log is a side effect we cannot easily assert.
	Config = GetDefaultConfig()
	prevInternal := IsInternalCall
	IsInternalCall = false
	t.Cleanup(func() { IsInternalCall = prevInternal })

	LoadFromEnv()
}

func TestLoad(t *testing.T) {
	// Load must reset to defaults then apply env. With no env vars set,
	// the result equals GetDefaultConfig().
	withEnv(t, map[string]string{
		"LISTEN_IP": "10.0.0.1",
	})
	prevInternal := IsInternalCall
	IsInternalCall = true
	t.Cleanup(func() { IsInternalCall = prevInternal })

	Load()

	if Config.ListenHost != "10.0.0.1" {
		t.Errorf("ListenHost = %q", Config.ListenHost)
	}
	if Config.Iperf3StartPort != 30000 {
		t.Errorf("default port not restored before LoadFromEnv; got %d", Config.Iperf3StartPort)
	}
}
