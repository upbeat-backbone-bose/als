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
	ClientIP               string   `json:"my_ip"`
	Location               string   `json:"location"`
	PublicIPv4             string   `json:"public_ipv4"`
	PublicIPv6             string   `json:"public_ipv6"`
	SpeedtestFileList      []string `json:"speedtest_files"`
	SponsorMessage         string   `json:"sponsor_message"`
	FeaturePing            bool     `json:"feature_ping"`
	FeatureShell           bool     `json:"feature_shell"`
	FeatureLibrespeed      bool     `json:"feature_librespeed"`
	FeatureFileSpeedtest   bool     `json:"feature_filespeedtest"`
	FeatureSpeedtestDotNet bool     `json:"feature_speedtest_dot_net"`
	FeatureIperf3          bool     `json:"feature_iperf3"`
	FeatureMTR             bool     `json:"feature_mtr"`
	FeatureTraceroute      bool     `json:"feature_traceroute"`
	FeatureIfaceTraffic    bool     `json:"feature_iface_traffic"`
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

// keepaliveInterval is the cadence at which Handle emits an SSE comment
// frame ("\n:keepalive\n\n") so that proxy / TCP idle timeouts do not
// close an otherwise-idle stream mid-chunk. The production value is
// 15s; tests override keepaliveIntervalForTest to drive the cadence
// down to a few milliseconds so they can observe keepalive frames.
const keepaliveInterval = 15 * time.Second

// keepaliveIntervalForTest, when non-zero, replaces keepaliveInterval
// for the duration of a test. Production code never sets it.
var keepaliveIntervalForTest time.Duration

func resolveKeepaliveInterval() time.Duration {
	if keepaliveIntervalForTest > 0 {
		return keepaliveIntervalForTest
	}
	return keepaliveInterval
}

func Handle(c *gin.Context) {
	// Resume path: if the client sends ?resume=<sessionId> for a
	// session that is still live, reuse it instead of minting a new
	// UUID. This is what keeps long-running operations (Librespeed
	// upload, IPerf3, shell websockets) alive across the ~30s SSE
	// reconnects that browsers and intermediate proxies impose on
	// idle EventSource streams. The old SSE handler is asked to
	// exit via Close(); its defer uses IsCurrent to avoid removing
	// the replacement entry that this handler is about to install.
	resumeID := c.Query("resume")
	var sessionID string
	var clientSession *client.ClientSession
	if resumeID != "" {
		if old, ok := client.GetClient(resumeID); ok {
			old.Close()
			client.RemoveClient(resumeID)
			clientSession = &client.ClientSession{
				Channel:   make(chan *client.Message, 64),
				CreatedAt: time.Now(),
			}
			client.AddClient(resumeID, clientSession)
			sessionID = resumeID
		}
	}
	if clientSession == nil {
		sessionID = uuid.New().String()
		clientSession = &client.ClientSession{
			Channel:   make(chan *client.Message, 64),
			CreatedAt: time.Now(),
		}
		client.AddClient(sessionID, clientSession)
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	clientSession.SetContext(ctx)
	defer func() {
		cancel()
		// Only remove the map entry if we are still the live handler
		// for this id. The resume path installs a replacement above
		// before the old handler's defer fires; without this guard
		// the stale handler would yank the new entry out from under
		// its replacement.
		if client.IsCurrent(sessionID, clientSession) {
			client.RemoveClient(sessionID)
		}
	}()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Tell nginx (and similar proxies) to forward our response
	// immediately instead of buffering it -- otherwise keepalive
	// frames can sit in the proxy buffer and never reach the client.
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.SSEvent("SessionId", sessionID)

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

	keepalive := time.NewTicker(resolveKeepaliveInterval())
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-clientSession.Done():
			// Another handler took over this session (the resume
			// path on /session) or the session was closed by a
			// higher-level teardown. Drop out so we don't keep
			// emitting on a writer that the new handler now owns.
			return
		case <-keepalive.C:
			// SSE comment frame ("\n:keepalive\n\n"). Browsers and
			// EventSource clients ignore lines that start with ":" so
			// this is a no-op from their POV, but the bytes still
			// flow through the chunked stream -- which is enough to
			// keep http.Server's WriteDeadline fresh and to keep any
			// upstream proxy from idle-timing out the connection.
			if _, err := c.Writer.WriteString(":keepalive\n\n"); err != nil {
				// Client socket is gone (or the writer has been
				// closed for some other reason). Stop the goroutine
				// so we no longer pin a session entry in the global
				// map.
				log.Default().Printf("session: keepalive write failed, closing: %v", err)
				return
			}
			c.Writer.Flush()
		case msg, ok := <-clientSession.Channel:
			if !ok {
				return
			}
			c.SSEvent(msg.Name, msg.Content)
			c.Writer.Flush()
		}
	}
}
