package http

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine *gin.Engine
	listen string
}

func CreateServer() *Server {
	gin.SetMode(gin.ReleaseMode)
	e := &Server{
		engine: gin.Default(),
		listen: ":8080",
	}
	return e
}

func (e *Server) GetEngine() *gin.Engine {
	return e.engine
}

func (e *Server) SetListen(listen string) {
	e.listen = listen
}

func (e *Server) Start() error {
	srv := &http.Server{
		Addr:    e.listen,
		Handler: e.engine,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		log.Println("Server exited")
	}()

	return srv.ListenAndServe()
}
