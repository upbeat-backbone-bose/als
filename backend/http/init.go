package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTP server timeout knobs. Defaults are chosen for a public-facing
// network-diagnostic server:
//
//   - ReadHeaderTimeout: 10s. Bounds the Slowloris-style "open a TCP
//     connection and trickle bytes one at a time" attack that would
//     otherwise keep file descriptors pinned forever.
//   - ReadTimeout: 30s. Caps the entire request body read.
//   - WriteTimeout: 10m. Bounds the overall response-write window.
//     Note: this is misleadingly named, it is *not* an idle / per-write
//     timeout -- it is a cap on the time from the first response byte
//     to the handler returning. It IS shared with long-lived
//     streaming endpoints (SSE, long file downloads).
//     Previously this was 30s, which empirically killed idle SSE
//     streams every 30s and surfaced to browsers as
//     ERR_INCOMPLETE_CHUNKED_ENCODING. We extend it to 10m so an
//     accident in the SSE keepalive cadence (every 15s by default)
//     cannot under any circumstance kill a single session early.
//     WebSocket still escapes this because it hijacks the connection
//     before the response writer goes live.
//   - IdleTimeout: 120s. Reaps keep-alive connections that go quiet
//     between sequential requests; it does not apply inside an
//     in-flight streaming response.
//
// gosec G112 flags any zero timeouts here. We deliberately keep
// WriteTimeout > 0 so the Slowloris regression guard
// TestServerTimeoutsAreSet in init_test.go stays green without a nolint.
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 10 * time.Minute
	httpIdleTimeout       = 120 * time.Second
)

type Server struct {
	engine     *gin.Engine
	listen     string
	httpServer *http.Server
	mu         sync.Mutex
}

func CreateServer() *Server {
	gin.SetMode(gin.ReleaseMode)
	e := &Server{
		engine: gin.Default(),
		listen: ":8080",
	}
	e.registerPprof()
	return e
}

func (e *Server) registerPprof() {
	g := e.engine.Group("/debug/pprof")
	g.GET("/", gin.WrapH(http.HandlerFunc(pprof.Index)))
	g.GET("/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
	g.GET("/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	g.POST("/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	g.GET("/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	g.GET("/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))
	g.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	g.GET("/block", gin.WrapH(pprof.Handler("block")))
	g.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	g.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	g.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	g.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}

func (e *Server) GetEngine() *gin.Engine {
	return e.engine
}

func (e *Server) SetListen(listen string) {
	e.listen = listen
}

// newHTTPServer builds the *http.Server that Start binds. Indirected
// through a package-level var so tests can inspect the configured
// timeouts without binding a port. Production code never overrides it.
func newHTTPServer(s *Server) *http.Server {
	return &http.Server{
		Addr:              s.listen,
		Handler:           s.engine,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

func (e *Server) Start() error {
	e.mu.Lock()
	if e.httpServer != nil {
		e.mu.Unlock()
		return errors.New("server already started")
	}
	e.httpServer = newHTTPServer(e)
	srv := e.httpServer
	e.mu.Unlock()
	return srv.ListenAndServe()
}

func (e *Server) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	srv := e.httpServer
	e.mu.Unlock()
	if srv == nil {
		return nil
	}
	log.Default().Println("Shutting down HTTP server...")
	return srv.Shutdown(ctx)
}
