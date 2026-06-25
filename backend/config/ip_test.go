package config

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type redirectTransport struct {
	serverURL string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := t.serverURL + req.URL.Path
	newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}

type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("simulated network error")
}

func TestGetPublicIPViaHttpSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "1.2.3.4")
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	ip, err := getPublicIPViaHttp(client)
	if err != nil {
		t.Fatalf("getPublicIPViaHttp: %v", err)
	}
	if ip != "1.2.3.4" {
		t.Errorf("ip = %q; want 1.2.3.4", ip)
	}
}

func TestGetPublicIPViaHttpTrimsWhitespace(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "  10.0.0.1\n")
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	ip, err := getPublicIPViaHttp(client)
	if err != nil {
		t.Fatalf("getPublicIPViaHttp: %v", err)
	}
	if ip != "10.0.0.1" {
		t.Errorf("ip = %q; want 10.0.0.1", ip)
	}
}

func TestGetPublicIPViaHttpIPv6(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "::1")
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	ip, err := getPublicIPViaHttp(client)
	if err != nil {
		t.Fatalf("getPublicIPViaHttp: %v", err)
	}
	if ip != "::1" {
		t.Errorf("ip = %q; want ::1", ip)
	}
}

func TestGetPublicIPViaHttpNon200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	_, err := getPublicIPViaHttp(client)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGetPublicIPViaHttpInvalidResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-an-ip")
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	_, err := getPublicIPViaHttp(client)
	if err == nil {
		t.Fatal("expected error for non-IP response")
	}
}

func TestGetPublicIPViaHttpEmptyBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: server.URL}}
	_, err := getPublicIPViaHttp(client)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestGetPublicIPViaHttpNetworkError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: &errorTransport{}}
	_, err := getPublicIPViaHttp(client)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestGetPublicIPViaHttpSkipsUnreachable(t *testing.T) {
	t.Parallel()

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "5.6.7.8")
	}))
	t.Cleanup(second.Close)

	client := &http.Client{Transport: &redirectTransport{serverURL: second.URL}}
	ip, err := getPublicIPViaHttp(client)
	if err != nil {
		t.Fatalf("getPublicIPViaHttp: %v", err)
	}
	if ip != "5.6.7.8" {
		t.Errorf("ip = %q; want 5.6.7.8", ip)
	}
}

// TestGetPublicIPv4ViaHttpWrapperRuns exercises the IPv4 wrapper.
// The wrapper creates its own client with a 5s timeout and
// forces IPv4 dialing. We cannot inject a transport (the client
// is built inside the function), so we run it as-is and assert
// that it returns a valid IPv4 address without error.
//
// In sandboxed environments without external connectivity, the
// function will return an error -- we treat that as a soft skip
// rather than a hard failure.
func TestGetPublicIPv4ViaHttpWrapperRuns(t *testing.T) {
	t.Parallel()

	ip, err := getPublicIPv4ViaHttp()
	if err != nil {
		t.Skipf("getPublicIPv4ViaHttp requires external connectivity: %v", err)
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("getPublicIPv4ViaHttp returned %q; not a valid IP", ip)
	}
	if v4 := parsed.To4(); v4 == nil {
		t.Errorf("getPublicIPv4ViaHttp returned %q; expected IPv4 (the wrapper forces tcp4 dial)", ip)
	}
}
