package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func LoadFromEnv(cfg *ALSConfig) {
	envVarsString := map[string]*string{
		"LISTEN_IP":       &cfg.ListenHost,
		"HTTP_PORT":       &cfg.ListenPort,
		"LOCATION":        &cfg.Location,
		"PUBLIC_IPV4":     &cfg.PublicIPv4,
		"PUBLIC_IPV6":     &cfg.PublicIPv6,
		"SPONSOR_MESSAGE": &cfg.SponsorMessage,
	}

	envVarsInt := map[string]*int{
		"UTILITIES_IPERF3_PORT_MIN": &cfg.Iperf3StartPort,
		"UTILITIES_IPERF3_PORT_MAX": &cfg.Iperf3EndPort,
	}

	envVarsBool := map[string]*bool{
		"DISPLAY_TRAFFIC":           &cfg.FeatureIfaceTraffic,
		"ENABLE_SPEEDTEST":          &cfg.FeatureLibrespeed,
		"UTILITIES_SPEEDTESTDOTNET": &cfg.FeatureSpeedtestDotNet,
		"UTILITIES_PING":            &cfg.FeaturePing,
		"UTILITIES_FAKESHELL":       &cfg.FeatureShell,
		"UTILITIES_IPERF3":          &cfg.FeatureIperf3,
		"UTILITIES_MTR":             &cfg.FeatureMTR,
	}

	for envVar, configField := range envVarsString {
		if v := os.Getenv(envVar); len(v) != 0 {
			*configField = v
		}
	}

	for envVar, configField := range envVarsInt {
		if v := os.Getenv(envVar); len(v) != 0 {
			v, err := strconv.Atoi(v)
			if err != nil {
				log.Printf("Invalid int value for %s: %v", envVar, err)
				continue
			}
			*configField = v
		}
	}

	for envVar, configField := range envVarsBool {
		if v := os.Getenv(envVar); len(v) != 0 {
			*configField = v == "true"
		}
	}

	if v := os.Getenv("SPEEDTEST_FILE_LIST"); len(v) != 0 {
		fileLists := strings.Split(v, " ")
		cfg.SpeedtestFileList = fileLists
	}

	if !IsInternalCall {
		log.Default().Println("Loading config from environment variables...")
	}
}
