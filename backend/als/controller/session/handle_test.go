package session

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
	"github.com/samlm0/als/v2/internal/testutil"
)

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
	testutil.WaitFor(t, time.Second, "Handle registered a session", func() bool {
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

	testutil.WaitFor(t, time.Second, "session removed", func() bool {
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
	testutil.WaitFor(t, time.Second, "Handle registered a session", func() bool {
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
	testutil.WaitFor(t, time.Second, "Ping event streamed", func() bool {
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
	testutil.WaitFor(t, time.Second, "Handle registered a session", func() bool {
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

// TestHandleStreamsKeepaliveCommentFrames verifies that the SSE handler
// emits a comment frame (":keepalive\n\n") on a fixed cadence even when
// no client-side events are pushed through the channel. Browsers /
// EventSource ignore those lines, but the bytes still flow through
// the chunked stream, which keeps http.Server's WriteDeadline fresh
// and prevents proxy idle timeouts from closing the connection.
//
// We drive the cadence down to 20ms via keepaliveIntervalForTest so
// the test completes in well under a second. We also assert the SSE
// response was flushed with X-Accel-Buffering turned off so an
// upstream nginx-style proxy will not buffer the keepalive bytes.
func TestHandleStreamsKeepaliveCommentFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	keepaliveIntervalForTest = 20 * time.Millisecond
	t.Cleanup(func() { keepaliveIntervalForTest = 0 })

	router := gin.New()
	router.GET("/session", Handle)

	// Run the handler with a timeout just slightly longer than the
	// span over which we expect several keepalive frames to arrive.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	bodyBuf := &threadSafeBuffer{}
	w := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: bodyBuf}

	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(w, reqWithCtx(ctx))
	}()
	<-done

	body := bodyBuf.String()

	// 250ms / 20ms = up to 12 ticks; allow plenty of slack.
	const minExpected = 3
	count := strings.Count(body, ":keepalive")
	if count < minExpected {
		t.Errorf("expected >= %d keepalive frames in body, got %d.\nbody:\n%s", minExpected, count, body)
	}

	// verify the proxy-bypass header is set so future proxies don't
	// sit on the keepalive frames.
	if got := w.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf(`X-Accel-Buffering = %q; want "no"`, got)
	}

	// keepalive bytes must still be interleaved with the regular SSE
	// prefix events, not in place of them.
	for _, want := range []string{"event:SessionId", "event:Config", "event:InterfaceCache"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in body; full body:\n%s", want, body)
		}
	}
}

// TestHandleResumeReusesSession verifies the resume path: a /session
// request carrying ?resume=<id> for a still-live session must reuse
// that id (not mint a new UUID) and install a fresh ClientSession
// entry in the global map. The old SSE handler must exit via the
// ClientSession.Done() signal triggered by the resume's Close();
// the IsCurrent guard in the first handler's defer must then skip
// RemoveClient so the replacement entry the second handler installs
// is not yanked out from under it.
func TestHandleResumeReusesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	// Short keepalive so the handlers actually unblock in test
	// time without us sleeping for the full 15s production value.
	keepaliveIntervalForTest = 20 * time.Millisecond
	t.Cleanup(func() { keepaliveIntervalForTest = 0 })

	router := gin.New()
	router.GET("/session", Handle)

	// --- 1st connection: open, capture the SessionId.
	firstBody := &threadSafeBuffer{}
	firstWriter := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: firstBody}
	firstCtx, firstCancel := context.WithCancel(context.Background())
	defer firstCancel()

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		router.ServeHTTP(firstWriter, reqWithCtx(firstCtx))
	}()

	// Wait for the first handler to publish its SessionId.
	var firstSessionID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if id := parseSSEEvent(t, firstBody.String(), "SessionId"); id != "" {
			firstSessionID = id
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if firstSessionID == "" {
		firstCancel()
		<-firstDone
		t.Fatalf("first /session never emitted SessionId; body=%q", firstBody.String())
	}

	// --- 2nd connection: ?resume=<originalID>.
	secondBody := &threadSafeBuffer{}
	secondWriter := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: secondBody}
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()

	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		req := httptest.NewRequest(http.MethodGet, "/session?resume="+firstSessionID, http.NoBody).WithContext(secondCtx)
		router.ServeHTTP(secondWriter, req)
	}()

	// The first handler must observe ClientSession.Done() closing
	// and return promptly.
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		firstCancel()
		<-firstDone
		t.Fatal("first /session handler did not exit after resume; takeover via Done() failed")
	}

	// Wait for the second handler to publish its SessionId.
	var secondSessionID string
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if id := parseSSEEvent(t, secondBody.String(), "SessionId"); id != "" {
			secondSessionID = id
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if secondSessionID == "" {
		secondCancel()
		<-secondDone
		t.Fatalf("second /session?resume=... never emitted SessionId; body=%q", secondBody.String())
	}

	if secondSessionID != firstSessionID {
		t.Errorf("resume minted a new session id: first=%q second=%q; want equal", firstSessionID, secondSessionID)
	}

	// The replacement entry must still be in the map. If the
	// first handler's defer had removed it (IsCurrent guard
	// broken), this lookup would miss and subsequent requests
	// using this session id would all 400.
	client.ClientsMu().RLock()
	entry, ok := client.Clients[firstSessionID]
	client.ClientsMu().RUnlock()
	if !ok {
		secondCancel()
		<-secondDone
		t.Fatalf("Clients[%q] missing after resume: the replacement entry was not preserved (IsCurrent guard likely broken)", firstSessionID)
	}
	if !client.IsCurrent(firstSessionID, entry) {
		secondCancel()
		<-secondDone
		t.Errorf("IsCurrent returned false for the live entry; IsCurrent guard is broken")
	}

	// Cancel the second request; the second handler exits and its
	// defer (IsCurrent guard passes, it IS current) removes the
	// entry. After both handlers are done the map must be empty
	// for this id.
	secondCancel()
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second /session handler did not exit on context cancel")
	}

	client.ClientsMu().RLock()
	_, stillThere := client.Clients[firstSessionID]
	client.ClientsMu().RUnlock()
	if stillThere {
		t.Errorf("Clients[%q] still present after both handlers exited", firstSessionID)
	}
}

// TestHandleResumeUnknownIdCreatesNew covers the fallback: a
// ?resume=<id> for an id that no longer exists must mint a fresh
// UUID and install a new entry. The client cannot accidentally
// inherit a stale session.
func TestHandleResumeUnknownIdCreatesNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stubConfigGetter(t, &config.ALSConfig{})

	keepaliveIntervalForTest = 20 * time.Millisecond
	t.Cleanup(func() { keepaliveIntervalForTest = 0 })

	router := gin.New()
	router.GET("/session", Handle)

	body := &threadSafeBuffer{}
	writer := &safeResponseRecorder{ResponseRecorder: httptest.NewRecorder(), buf: body}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodGet, "/session?resume=definitely-not-a-live-id", http.NoBody).WithContext(ctx)
		router.ServeHTTP(writer, req)
	}()

	// Wait for the handler to publish its SessionId and then
	// cancel so it exits cleanly.
	var newID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if id := parseSSEEvent(t, body.String(), "SessionId"); id != "" {
			newID = id
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if newID == "" {
		t.Fatalf("/session?resume=unknown never emitted SessionId; body=%q", body.String())
	}
	if newID == "definitely-not-a-live-id" {
		t.Errorf("expected a fresh UUID, got the unknown resume id back: %q", newID)
	}

	// Cleanup so the test's leftover entry doesn't leak into
	// other tests in the package.
	client.RemoveClient(newID)
}
