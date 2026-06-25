package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type locationTransport struct {
	serverURL string
	inner     http.RoundTripper
}

func (t *locationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	inner := t.inner
	if inner == nil {
		inner = http.DefaultTransport
	}
	newURL := t.serverURL + req.URL.Path
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return inner.RoundTrip(newReq)
}

// installTestTransport replaces http.DefaultTransport (and
// http.DefaultClient.Transport) with a transport that redirects
// requests to the supplied test server URL. The original transport
// is restored on test cleanup.
func installTestTransport(t *testing.T, serverURL string) {
	t.Helper()

	prevTransport := http.DefaultTransport
	prevClient := http.DefaultClient
	http.DefaultTransport = &locationTransport{
		serverURL: serverURL,
		inner:     prevTransport,
	}
	http.DefaultClient = &http.Client{Transport: http.DefaultTransport}

	t.Cleanup(func() {
		http.DefaultTransport = prevTransport
		http.DefaultClient = prevClient
	})
}

func TestUpdateLocationSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"city":"Shanghai","country_name":"China"}`)
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "Shanghai, China" {
		t.Errorf("Config.Location = %q; want %q", Config.Location, "Shanghai, China")
	}
}

func TestUpdateLocationInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "" {
		t.Errorf("Config.Location = %q; want empty on JSON parse failure", Config.Location)
	}
}

func TestUpdateLocationNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "" {
		t.Errorf("Config.Location = %q; want empty on non-200", Config.Location)
	}
}

func TestUpdateLocationMissingCity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"country_name":"China"}`)
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "" {
		t.Errorf("Config.Location = %q; want empty when city is missing", Config.Location)
	}
}

func TestUpdateLocationMissingCountry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"city":"Shanghai"}`)
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "" {
		t.Errorf("Config.Location = %q; want empty when country_name is missing", Config.Location)
	}
}

func TestUpdateLocationHTTPScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"city":"Beijing","country_name":"China"}`)
	}))
	t.Cleanup(server.Close)

	installTestTransport(t, server.URL)
	withConfig(t, &ALSConfig{})

	updateLocation()

	if Config.Location != "Beijing, China" {
		t.Errorf("Config.Location = %q; want %q", Config.Location, "Beijing, China")
	}
}
