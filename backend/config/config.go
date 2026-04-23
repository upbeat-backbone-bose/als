package config

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type ALSConfig struct {
	ListenHost string `json:"-"`
	ListenPort string `json:"-"`

	Location string `json:"location"`

	PublicIPv4 string `json:"public_ipv4"`
	PublicIPv6 string `json:"public_ipv6"`

	Iperf3StartPort int `json:"-"`
	Iperf3EndPort   int `json:"-"`

	SpeedtestFileList []string `json:"speedtest_files"`

	SponsorMessage string `json:"sponsor_message"`

	FeaturePing            bool `json:"feature_ping"`
	FeatureShell           bool `json:"feature_shell"`
	FeatureLibrespeed      bool `json:"feature_librespeed"`
	FeatureFileSpeedtest   bool `json:"feature_filespeedtest"`
	FeatureSpeedtestDotNet bool `json:"feature_speedtest_dot_net"`
	FeatureIperf3          bool `json:"feature_iperf3"`
	FeatureMTR             bool `json:"feature_mtr"`
	FeatureTraceroute      bool `json:"feature_traceroute"`
	FeatureIfaceTraffic    bool `json:"feature_iface_traffic"`
}

func NewConfig() *ALSConfig {
	return &ALSConfig{
		ListenHost:      "0.0.0.0",
		ListenPort:      "80",
		Location:        "",
		Iperf3StartPort: 30000,
		Iperf3EndPort:   31000,

		SpeedtestFileList: []string{"1MB", "10MB", "100MB", "1GB", "100GB"},
		PublicIPv4:        "",
		PublicIPv6:        "",

		FeaturePing:            true,
		FeatureShell:           true,
		FeatureLibrespeed:      true,
		FeatureFileSpeedtest:   true,
		FeatureSpeedtestDotNet: true,
		FeatureIperf3:          true,
		FeatureMTR:             true,
		FeatureTraceroute:      true,
		FeatureIfaceTraffic:    true,
	}
}

func (c *ALSConfig) Load() {
	LoadFromEnv(c)
}

func (c *ALSConfig) LoadWebConfig() error {
	c.Load()
	if err := c.LoadSponsorMessage(); err != nil {
		return err
	}
	log.Default().Println("Loading config for web services...")

	_, err := exec.LookPath("iperf3")
	if err != nil {
		log.Default().Println("WARN: Disable iperf3 due to not found")
		c.FeatureIperf3 = false
	}

	if c.PublicIPv4 == "" && c.PublicIPv6 == "" {
		go func() {
			updatePublicIP()
			if c.Location == "" {
				updateLocation()
			}
		}()
	}

	return nil
}

func (c *ALSConfig) LoadSponsorMessage() error {
	if c.SponsorMessage == "" {
		return nil
	}

	log.Default().Println("Loading sponsor message...")

	if _, err := os.Stat(c.SponsorMessage); err == nil {
		content, err := os.ReadFile(c.SponsorMessage)
		if err == nil {
			c.SponsorMessage = string(content)
			return nil
		}
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(c.SponsorMessage)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Default().Println("ERROR: Failed to load sponsor message.")
			return nil
		}
		content, err := io.ReadAll(resp.Body)
		if err == nil {
			log.Default().Println("Loaded sponsor message from url.")
			c.SponsorMessage = string(content)
			return nil
		}
	}

	log.Default().Println("ERROR: Failed to load sponsor message.")
	return nil
}
