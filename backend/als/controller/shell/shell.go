package shell

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

// allowedOrigins holds the explicit allow-list used by checkOrigin to
// decide whether a WebSocket upgrade request is allowed.
//
// It is populated once at startup from configuration (public IPv4/IPv6,
// listen host, ALLOWED_ORIGINS env) and read-only thereafter. A nil or
// empty list disables the WebSocket endpoint outright, which is the
// safe default -- if the operator did not opt in, no remote shell is
// exposed.
//
// The previous implementation accepted requests with an empty Origin
// header and trusted the Host header for cross-origin comparison. Both
// behaviours are exploitable (CSWSH and Host-header smuggling). The
// allow-list replaces both. See Issue #1.
var allowedOrigins []string

// setAllowedOriginsForTest overrides the package-level allow-list for
// tests. Returns a cleanup function that restores the previous value.
// Production code must not call this.
func setAllowedOriginsForTest(origins []string) func() {
	prev := allowedOrigins
	allowedOrigins = append([]string(nil), origins...)
	return func() { allowedOrigins = prev }
}

// buildAllowedOrigins computes the default allow-list from the running
// server's own addresses plus any operator-supplied ALLOWED_ORIGINS
// entries. It is exposed at package scope so tests can exercise it
// without mutating global state.
//
// Duplicates are removed and the result is lower-cased for case-insensitive
// matching. Bare hosts ("example.com") and hosts with explicit ports
// ("example.com:8080") are kept as separate entries: callers that need
// port-stripped matching should strip the port themselves. The checkOrigin
// helper does that below.
func buildAllowedOrigins(cfg *config.ALSConfig) []string {
	seen := make(map[string]struct{})
	var out []string

	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	add(cfg.PublicIPv4)
	add(cfg.PublicIPv6)
	add(cfg.ListenHost)

	for _, extra := range cfg.AllowedOrigins {
		add(extra)
	}

	return out
}

// stripPort returns host with any ":port" suffix removed. IPv6 literals
// in square brackets ("[::1]:8080") are handled correctly. The empty
// string is returned unchanged so an unset host does not match every
// request.
func stripPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		// IPv6 literal: bracketed host with optional ":port".
		if i := strings.LastIndex(host, "]"); i >= 0 {
			return strings.ToLower(host[:i+1])
		}
		return strings.ToLower(host)
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		return strings.ToLower(host[:i])
	}
	return strings.ToLower(host)
}

// checkOrigin is the policy applied to WebSocket upgrade requests for
// the fake shell endpoint.
//
// Policy:
//
//  1. Missing Origin is rejected. Browsers always send Origin on a
//     cross-origin or even same-origin WebSocket request, so a missing
//     Origin header indicates a non-browser client (curl, native app,
//     attacker script) and is not a legitimate signal of trust.
//
//  2. Origin must appear on the explicit allow-list populated at
//     startup. The allow-list is keyed on host (port stripped), so
//     "https://als.example.com:8443" matches a configured entry of
//     "als.example.com" without forcing operators to enumerate every
//     port.
//
//  3. The Host header is never consulted: the previous implementation
//     trusted r.Host which is forgeable behind a misconfigured reverse
//     proxy and made CSWSH trivial.
//
// If allowedOrigins is empty (operator did not opt in), every upgrade
// is rejected -- this is the safe default for a feature that gives
// remote shell access.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}

	originHost := stripPort(u.Host)
	if originHost == "" {
		return false
	}

	if len(allowedOrigins) == 0 {
		return false
	}

	for _, allowed := range allowedOrigins {
		allowedHost := stripPort(allowed)
		if allowedHost != "" && originHost == allowedHost {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     checkOrigin,
}

// InitAllowedOrigins builds the WebSocket Origin allow-list from cfg
// and stores it on the package. Call this once at startup, after
// config.LoadWebConfig. It is safe to call again to refresh the list
// (e.g. in tests).
func InitAllowedOrigins(cfg *config.ALSConfig) {
	allowedOrigins = buildAllowedOrigins(cfg)
}

// AllowedOriginsSnapshot returns a copy of the current allow-list.
// Callers (e.g. startup logging) must not mutate the returned slice.
func AllowedOriginsSnapshot() []string {
	out := make([]string, len(allowedOrigins))
	copy(out, allowedOrigins)
	return out
}

func HandleNewShell(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = conn.Close() }()
	v, _ := c.Get("clientSession")
	clientSession, ok := client.SessionFromContext(v)
	if !ok {
		_ = conn.Close()
		return
	}
	handleNewConnection(conn, clientSession, c)
}

func handleNewConnection(conn *websocket.Conn, session *client.ClientSession, ginC *gin.Context) {
	ctx, cancel := context.WithCancel(session.GetContext(ginC.Request.Context()))
	defer cancel()

	ex, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, ex, "--shell") // #nosec G204 command is fixed to current binary
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return
	}
	defer func() { _ = ptmx.Close() }()

	// context aware
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// cmd -> websocket
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				break
			}
			if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
				break
			}
		}
	}()

	// websocket -> cmd
	go func() {
		defer cancel()
		for {
			_, buf, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if len(buf) < 1 {
				continue
			}
			index := string(buf[:1])
			switch index {
			case "1":
				// normal input
				if _, err := ptmx.Write(buf[1:]); err != nil {
					return
				}
			case "2":
				// win resize
				args := strings.Split(string(buf[1:]), ";")
				if len(args) < 2 {
					continue
				}
				h, errH := strconv.Atoi(args[0])
				w, errW := strconv.Atoi(args[1])
				if errH != nil || errW != nil {
					continue
				}
				if h <= 0 || h > int(^uint16(0)) || w <= 0 || w > int(^uint16(0)) {
					continue
				}
				if err := pty.Setsize(ptmx, &pty.Winsize{
					Rows: uint16(h),
					Cols: uint16(w),
				}); err != nil {
					return
				}
			}
		}
	}()
	if err := cmd.Wait(); err != nil && !errors.Is(err, syscall.ECHILD) {
		fmt.Println("shell command exited with error:", err)
	}
}
