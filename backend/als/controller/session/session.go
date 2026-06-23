package session

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
)

// ClientConfig is the minimal projection of ALSConfig that the UI consumes.
// Internal fields (listen_host/port, iperf3 port range) are intentionally
// excluded; the UI must never see them.
type ClientConfig struct {
	ClientIP              string   `json:"my_ip"`
	Location              string   `json:"location"`
	PublicIPv4            string   `json:"public_ipv4"`
	PublicIPv6            string   `json:"public_ipv6"`
	SpeedtestFileList     []string `json:"speedtest_files"`
	SponsorMessage        string   `json:"sponsor_message"`
	FeaturePing           bool     `json:"feature_ping"`
	FeatureShell          bool     `json:"feature_shell"`
	FeatureLibrespeed     bool     `json:"feature_librespeed"`
	FeatureFileSpeedtest  bool     `json:"feature_filespeedtest"`
	FeatureSpeedtestDotNet bool    `json:"feature_speedtest_dot_net"`
	FeatureIperf3         bool     `json:"feature_iperf3"`
	FeatureMTR            bool     `json:"feature_mtr"`
	FeatureTraceroute     bool     `json:"feature_traceroute"`
	FeatureIfaceTraffic   bool     `json:"feature_iface_traffic"`
}

func buildClientConfig(cfg *config.ALSConfig, clientIP string) ClientConfig {
	return ClientConfig{
		ClientIP:               clientIP,
		Location:               cfg.Location,
		PublicIPv4:             cfg.PublicIPv4,
		PublicIPv6:             cfg.PublicIPv6,
		SpeedtestFileList:      cfg.SpeedtestFileList,
		SponsorMessage:         cfg.SponsorMessage,
		FeaturePing:            cfg.FeaturePing,
		FeatureShell:           cfg.FeatureShell,
		FeatureLibrespeed:      cfg.FeatureLibrespeed,
		FeatureFileSpeedtest:   cfg.FeatureFileSpeedtest,
		FeatureSpeedtestDotNet: cfg.FeatureSpeedtestDotNet,
		FeatureIperf3:          cfg.FeatureIperf3,
		FeatureMTR:             cfg.FeatureMTR,
		FeatureTraceroute:      cfg.FeatureTraceroute,
		FeatureIfaceTraffic:    cfg.FeatureIfaceTraffic,
	}
}

// configGetter is overridable in tests; production reads config.Config directly.
var configGetter = func() *config.ALSConfig { return config.Config }

func Handle(c *gin.Context) {
	uuid := uuid.New().String()
	channel := make(chan *client.Message, 64)
	clientSession := &client.ClientSession{
		Channel:   channel,
		CreatedAt: time.Now(),
	}
	client.AddClient(uuid, clientSession)
	ctx, cancel := context.WithCancel(c.Request.Context())
	clientSession.SetContext(ctx)
	defer func() {
		cancel()
		client.RemoveClient(uuid)
	}()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.SSEvent("SessionId", uuid)

	clientCfg := buildClientConfig(configGetter(), c.ClientIP())
	configJson, err := json.Marshal(clientCfg)
	if err != nil {
		log.Default().Printf("session: marshal client config failed: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.SSEvent("Config", string(configJson))
	c.Writer.Flush()

	interfaceCacheJson, err := json.Marshal(timer.GetInterfaceCachesSnapshot())
	if err != nil {
		log.Default().Printf("session: marshal interface cache failed: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.SSEvent("InterfaceCache", string(interfaceCacheJson))
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-channel:
			if !ok {
				return
			}
			c.SSEvent(msg.Name, msg.Content)
			c.Writer.Flush()
		}
	}
}