package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_StringFields(t *testing.T) {
	os.Clearenv()
	os.Setenv("LISTEN_IP", "192.168.1.1")
	os.Setenv("HTTP_PORT", "8080")
	os.Setenv("LOCATION", "TestLocation")
	
	cfg := NewConfig()
	LoadFromEnv(cfg)
	
	if cfg.ListenHost != "192.168.1.1" {
		t.Errorf("Expected ListenHost 192.168.1.1, got %s", cfg.ListenHost)
	}
	if cfg.ListenPort != "8080" {
		t.Errorf("Expected ListenPort 8080, got %s", cfg.ListenPort)
	}
	if cfg.Location != "TestLocation" {
		t.Errorf("Expected Location TestLocation, got %s", cfg.Location)
	}
}

func TestLoadFromEnv_IntFields(t *testing.T) {
	os.Clearenv()
	os.Setenv("UTILITIES_IPERF3_PORT_MIN", "5000")
	os.Setenv("UTILITIES_IPERF3_PORT_MAX", "6000")
	
	cfg := NewConfig()
	LoadFromEnv(cfg)
	
	if cfg.Iperf3StartPort != 5000 {
		t.Errorf("Expected Iperf3StartPort 5000, got %d", cfg.Iperf3StartPort)
	}
	if cfg.Iperf3EndPort != 6000 {
		t.Errorf("Expected Iperf3EndPort 6000, got %d", cfg.Iperf3EndPort)
	}
}

func TestLoadFromEnv_IntFields_InvalidValue(t *testing.T) {
	os.Clearenv()
	os.Setenv("UTILITIES_IPERF3_PORT_MIN", "invalid")
	
	cfg := NewConfig()
	cfg.Iperf3StartPort = 30000 // default value
	LoadFromEnv(cfg)
	
	if cfg.Iperf3StartPort != 30000 {
		t.Errorf("Expected Iperf3StartPort to keep default 30000, got %d", cfg.Iperf3StartPort)
	}
}

func TestLoadFromEnv_BoolFields(t *testing.T) {
	os.Clearenv()
	os.Setenv("UTILITIES_PING", "false")
	os.Setenv("UTILITIES_IPERF3", "true")
	
	cfg := NewConfig()
	LoadFromEnv(cfg)
	
	if cfg.FeaturePing != false {
		t.Errorf("Expected FeaturePing false, got %v", cfg.FeaturePing)
	}
	if cfg.FeatureIperf3 != true {
		t.Errorf("Expected FeatureIperf3 true, got %v", cfg.FeatureIperf3)
	}
}

func TestLoadFromEnv_BoolFields_InvalidValue(t *testing.T) {
	os.Clearenv()
	os.Setenv("UTILITIES_PING", "invalid")
	
	cfg := NewConfig()
	cfg.FeaturePing = true // default value
	LoadFromEnv(cfg)
	
	if cfg.FeaturePing != true {
		t.Errorf("Expected FeaturePing to keep default true, got %v", cfg.FeaturePing)
	}
}

func TestLoadFromEnv_SpeedtestFileList(t *testing.T) {
	os.Clearenv()
	os.Setenv("SPEEDTEST_FILE_LIST", "1MB 10MB 100MB")
	
	cfg := NewConfig()
	LoadFromEnv(cfg)
	
	expected := []string{"1MB", "10MB", "100MB"}
	if len(cfg.SpeedtestFileList) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(cfg.SpeedtestFileList))
	}
	for i, f := range expected {
		if cfg.SpeedtestFileList[i] != f {
			t.Errorf("Expected file %d to be %s, got %s", i, f, cfg.SpeedtestFileList[i])
		}
	}
}

func TestLoadFromEnv_EmptyEnv(t *testing.T) {
	os.Clearenv()
	
	cfg := NewConfig()
	defaultPort := cfg.ListenPort
	LoadFromEnv(cfg)
	
	if cfg.ListenPort != defaultPort {
		t.Errorf("Expected ListenPort to keep default %s, got %s", defaultPort, cfg.ListenPort)
	}
}
