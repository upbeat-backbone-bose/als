package session

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

// waitFor spins until cond returns true or timeout elapses. Replaces
// the legacy time.Sleep-based polling. See als/client/wait_helpers_test.go
// for the canonical version.
//
// linter's per-file analysis can't see across files.
//
//nolint:unparam // each call site picks a different timeout; the
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitFor timed out after %v: %s", timeout, msg)
		}
		select {
		case <-tick.C:
		default:
			runtime.Gosched()
		}
	}
}

// stubConfigGetter replaces configGetter for the duration of t.
func stubConfigGetter(t *testing.T, cfg *config.ALSConfig) {
	t.Helper()
	prev := configGetter
	configGetter = func() *config.ALSConfig { return cfg }
	t.Cleanup(func() { configGetter = prev })
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

	req := httptest.NewRequest(http.MethodGet, "/session", http.NoBody).WithContext(ctx)
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

	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	bodyBuf := &threadSafeBuffer{}
	w := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: bodyBuf}

	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(w, reqWithCtx(parentCtx))
	}()

	// Phase 1: the session must be registered in the global map.
	waitFor(t, time.Second, "Handle registered a session", func() bool {
		client.ClientsMu().RLock()
		defer client.ClientsMu().RUnlock()
		return len(client.Clients) > 0
	})

	// Phase 2: cancel the request, wait for the handler to return, and
	// verify the session is no longer in the global map (defer fires).
	parentCancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handle did not exit after ctx cancel")
	}

	waitFor(t, time.Second, "session removed", func() bool {
		client.ClientsMu().RLock()
		n := len(client.Clients)
		client.ClientsMu().RUnlock()
		return n == 0
	})

	if !strings.Contains(bodyBuf.String(), "event:SessionId") {
		t.Errorf("SessionId event missing from body: %s", bodyBuf.String())
	}
}

// TestHandleStreamsCustomEvent verifies that a message pushed onto
// the ClientSession channel after Handle has registered the session
// is delivered to the SSE response.
func TestHandleStreamsCustomEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	router := gin.New()
	router.GET("/session", Handle)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Use a safe body wrapper since httptest.ResponseRecorder is not
	// safe for concurrent reads.
	bodyBuf := &threadSafeBuffer{}
	w := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: bodyBuf}

	// Run the handler on a goroutine; once it has registered the
	// session, push a message through the channel and observe the
	// SSE body.
	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(w, reqWithCtx(ctx))
	}()

	// Wait until Handle has registered the session in the global map.
	var session *client.ClientSession
	waitFor(t, time.Second, "Handle registered a session", func() bool {
		client.ClientsMu().RLock()
		defer client.ClientsMu().RUnlock()
		for _, s := range client.Clients {
			if s != nil {
				session = s
				return true
			}
		}
		return false
	})

	// Push a message. The handler picks it up and emits it as an SSE
	// event named after msg.Name.
	select {
	case session.Channel <- &client.Message{Name: "Ping", Content: "pong"}:
	case <-time.After(time.Second):
		t.Fatal("could not enqueue message -- channel full or session gone")
	}

	// Wait until the SSE body contains our event.
	waitFor(t, time.Second, "Ping event streamed", func() bool {
		body := bodyBuf.String()
		return strings.Contains(body, "event:Ping") &&
			strings.Contains(body, "data:pong")
	})

	// Cancel to let the handler return.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handle did not exit after ctx cancel")
	}
}

// TestHandleExitsWhenChannelCloses covers the path where Handle
// receives a zero-value message on its channel (channel closed)
// and returns without writing further SSE events.
func TestHandleExitsWhenChannelCloses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	r := gin.New()
	r.GET("/session", Handle)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bodyBuf := &threadSafeBuffer{}
	w := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: bodyBuf}

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.ServeHTTP(w, reqWithCtx(ctx))
	}()

	// Wait for the session to be registered, then close its channel.
	var session *client.ClientSession
	waitFor(t, time.Second, "Handle registered a session", func() bool {
		client.ClientsMu().RLock()
		defer client.ClientsMu().RUnlock()
		for _, s := range client.Clients {
			if s != nil {
				session = s
				return true
			}
		}
		return false
	})
	close(session.Channel)

	// Handle should observe the closed channel and return.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handle did not exit after channel close")
	}

	// SessionId and Config events must have been written before the
	// exit; the body must contain them.
	body := bodyBuf.String()
	if !strings.Contains(body, "event:SessionId") {
		t.Error("SessionId event missing")
	}
	if !strings.Contains(body, "event:Config") {
		t.Error("Config event missing")
	}
}

// threadSafeBuffer wraps bytes.Buffer with a mutex so the test thread
// can snapshot the SSE response body while the handler goroutine
// continues to write.
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// safeResponseRecorder captures SSE writes through a thread-safe buffer
// while delegating other methods to the standard httptest recorder.
//
// Only Write and WriteString are wrapped. Other methods (Header, Code,
// Flush) still go to the embedded *httptest.ResponseRecorder, which is
// NOT safe for concurrent access. Tests using this wrapper must only
// assert on body content; do not assert on Header or Code after the
// handler goroutine has started writing.
type safeResponseRecorder struct {
	*httptest.ResponseRecorder
	buf *threadSafeBuffer
}

func (r *safeResponseRecorder) Write(p []byte) (int, error) { return r.buf.Write(p) }
func (r *safeResponseRecorder) WriteString(s string) (int, error) {
	return r.buf.Write([]byte(s))
}

func reqWithCtx(ctx context.Context) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/session", http.NoBody).WithContext(ctx)
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
