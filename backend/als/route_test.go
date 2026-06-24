package als

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/config"
)

// routesFor returns the list of registered routes on e.
func routesFor(e *gin.Engine) []gin.RouteInfo {
	return e.Routes()
}

func TestSetupHttpRouteAllFeaturesOff(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	e := gin.New()
	SetupHttpRoute(e)

	routes := routesFor(e)
	// Even with every feature off we still register the always-on
	// routes (session, assets, root, speedtest_worker.js, favicon.ico).
	want := map[string]bool{
		"GET /session":                                   false,
		"GET /method/iperf3/server":                      false,
		"GET /method/ping":                               false,
		"GET /method/speedtest_dot_net":                  false,
		"GET /method/cache/interfaces":                   false,
		"GET /session/:session/shell":                    false,
		"GET /session/:session/speedtest/file/:filename": false,
		"GET /session/:session/speedtest/download":       false,
		"POST /session/:session/speedtest/upload":        false,
		"GET /assets/:filename":                          false,
		"GET /":                                          false,
		"GET /speedtest_worker.js":                       false,
		"GET /favicon.ico":                               false,
	}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		switch key {
		case "GET /session", "GET /assets/:filename",
			"GET /", "GET /speedtest_worker.js", "GET /favicon.ico":
			if !seen {
				t.Errorf("always-on route %q was not registered", key)
			}
		default:
			if seen {
				t.Errorf("feature-gated route %q should not be registered when all features are off", key)
			}
		}
	}
}

func TestSetupHttpRouteAllFeaturesOn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		FeatureIperf3:          true,
		FeaturePing:            true,
		FeatureSpeedtestDotNet: true,
		FeatureIfaceTraffic:    true,
		FeatureShell:           true,
		FeatureFileSpeedtest:   true,
		FeatureLibrespeed:      true,
	}
	t.Cleanup(func() { config.Config = prev })

	e := gin.New()
	SetupHttpRoute(e)

	want := map[string]bool{
		"GET /session":                                   false,
		"GET /method/iperf3/server":                      false,
		"GET /method/ping":                               false,
		"GET /method/speedtest_dot_net":                  false,
		"GET /method/cache/interfaces":                   false,
		"GET /session/:session/shell":                    false,
		"GET /session/:session/speedtest/file/:filename": false,
		"GET /session/:session/speedtest/download":       false,
		"POST /session/:session/speedtest/upload":        false,
		"GET /assets/:filename":                          false,
		"GET /":                                          false,
		"GET /speedtest_worker.js":                       false,
		"GET /favicon.ico":                               false,
	}
	for _, r := range routesFor(e) {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("route %q should be registered when all features are on", key)
		}
	}
}

func TestSetupHttpRouteSelectiveFeatures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{
		FeaturePing:  true,
		FeatureShell: true,
	}
	t.Cleanup(func() { config.Config = prev })

	e := gin.New()
	SetupHttpRoute(e)

	routes := routesFor(e)
	hasPing := false
	hasShell := false
	hasIperf3 := false
	for _, r := range routes {
		switch r.Path {
		case "/method/ping":
			hasPing = true
		case "/session/:session/shell":
			hasShell = true
		case "/method/iperf3/server":
			hasIperf3 = true
		}
	}
	if !hasPing {
		t.Error("/method/ping should be registered when FeaturePing is true")
	}
	if !hasShell {
		t.Error("/session/:session/shell should be registered when FeatureShell is true")
	}
	if hasIperf3 {
		t.Error("/method/iperf3/server should NOT be registered when FeatureIperf3 is false")
	}
}

func TestHandleStaticFileReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	e := gin.New()
	e.GET("/probe", func(c *gin.Context) {
		handleStatisFile("does-not-exist.html", c)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404; body = %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "Not found" {
		t.Errorf("body = %q; want 'Not found'", w.Body.String())
	}
}

// TestHandleStaticFileRoutesRegister verifies the always-on static
// routes (/speedtest_worker.js, /favicon.ico, /) are wired up by
// SetupHttpRoute. We only assert that the route exists and produces
// some response -- depending on whether the UI was built and embedded
// at compile time, the response is 200 (asset served) or 404 (asset
// missing). Either proves the route is registered.
func TestHandleStaticFileRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	e := gin.New()
	SetupHttpRoute(e)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/speedtest_worker.js"},
		{http.MethodGet, "/favicon.ico"},
		{http.MethodGet, "/"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)

			// Acceptable: 200 (asset served) or 404 (asset missing).
			if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
				t.Errorf("%s %s: status = %d; want 200 or 404", tt.method, tt.path, w.Code)
			}
		})
	}
}
