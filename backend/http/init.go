package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/gin-gonic/gin"
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

func (e *Server) Start() error {
	e.mu.Lock()
	if e.httpServer != nil {
		e.mu.Unlock()
		return errors.New("server already started")
	}
	e.httpServer = &http.Server{
		Addr:    e.listen,
		Handler: e.engine,
	}
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
