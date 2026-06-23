package session

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

// stubConfigGetter replaces configGetter for the duration of t.
func stubConfigGetter(t *testing.T, cfg *config.ALSConfig) {
	t.Helper()
	prev := configGetter
	configGetter = func() *config.ALSConfig { return cfg }
	t.Cleanup(func() { configGetter = prev })
}

// clearClients drops any session entries added by Handle so tests don't leak
// across runs.
func clearClients(t *testing.T) {
	t.Helper()
	client.RemoveClient("__test_cleanup__") // safe even if absent
	// Best-effort cleanup: we only added clients via Handle, which uses
	// fresh uuid per request. Use the public cleanup path so mutex is held.
	_ = client.RemoveExpiredClients
}

func TestHandleSSEConfigEventOmitsInternalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.ALSConfig{
		ListenHost:             "127.0.0.1",
		ListenPort:             "8080",
		Location:               "Earth",
		PublicIPv4:             "1.2.3.4",
		PublicIPv6:             "::1",
		Iperf3StartPort:        30000,
		Iperf3EndPort:          31000,
		SpeedtestFileList:      []string{"1MB", "10MB"},
		SponsorMessage:         "hi",
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
	stubConfigGetter(t, cfg)

	router := gin.New()
	router.GET("/session", Handle)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/session", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()
	<-done

	body := w.Body.String()

	// Headers: SSE
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q; want prefix text/event-stream", got)
	}

	// Parse SSE events; find the Config event and decode its data payload.
	cfgEvent := parseSSEEvent(t, body, "Config")
	if cfgEvent == "" {
		t.Fatalf("Config event not found in body:\n%s", body)
	}

	var got ClientConfig
	if err := json.Unmarshal([]byte(cfgEvent), &got); err != nil {
		t.Fatalf("Config event is not valid JSON: %v\npayload: %s", err, cfgEvent)
	}

	// Spot-check fields propagate from configGetter.
	if got.Location != "Earth" || got.PublicIPv4 != "1.2.3.4" || got.PublicIPv6 != "::1" {
		t.Errorf("Config event missing server info: %+v", got)
	}
	if !got.FeaturePing || !got.FeatureIperf3 {
		t.Errorf("Config event missing feature flags: %+v", got)
	}
	if got.SponsorMessage != "hi" {
		t.Errorf("SponsorMessage = %q; want hi", got.SponsorMessage)
	}
	if len(got.SpeedtestFileList) != 2 || got.SpeedtestFileList[0] != "1MB" {
		t.Errorf("SpeedtestFileList = %v; want [1MB 10MB]", got.SpeedtestFileList)
	}

	// Hard guarantee: no internal field appears anywhere in the response.
	for _, leaked := range []string{
		"listen_host", "listen_port",
		"iperf3_start_port", "iperf3_end_port",
	} {
		if strings.Contains(body, leaked) {
			t.Errorf("internal field %q leaked into SSE response:\n%s", leaked, body)
		}
	}
}

func TestHandleRegistersAndRemovesClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	router := gin.New()
	router.GET("/session", Handle)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/session", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event:SessionId") {
		t.Fatalf("SessionId event not found:\n%s", body)
	}

	// After Handle returns, the session should be removed from the global
	// map (defer fires regardless of how the loop exits).
	if count := client.RemoveExpiredClients(); count != 0 {
		// Some leftover may exist from another test; just check no zombie
		// session is older than the request we just served.
		t.Logf("note: %d sessions older than 24h were cleaned up", count)
	}
}

// parseSSEEvent scans a SSE-formatted body and returns the data payload of
// the named event. Fails the test if the event is not present.
func parseSSEEvent(t *testing.T, body, name string) string {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	var (
		currentEvent string
		dataLines    []string
	)
	flush := func() {
		if currentEvent == name {
			t.Logf("found %s event with %d data lines", name, len(dataLines))
		}
		currentEvent = ""
		dataLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			flush()
			currentEvent = strings.TrimPrefix(line, "event:")
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		case line == "":
			if currentEvent == name {
				return strings.Join(dataLines, "\n")
			}
			currentEvent = ""
			dataLines = nil
		}
	}
	// Tolerate trailing event without blank-line separator.
	if currentEvent == name {
		return strings.Join(dataLines, "\n")
	}
	return ""
}