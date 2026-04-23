package als

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
	alsHttp "github.com/samlm0/als/v2/http"
)

type Application struct {
	config     *config.ALSConfig
	clientMgr  *client.ClientManager
	httpServer *alsHttp.Server
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewApplication(cfg *config.ALSConfig) *Application {
	ctx, cancel := context.WithCancel(context.Background())
	return &Application{
		config:     cfg,
		clientMgr:  client.NewClientManager(),
		httpServer: alsHttp.CreateServer(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (app *Application) Init() error {
	app.httpServer.SetListen(app.config.ListenHost + ":" + app.config.ListenPort)

	SetupHttpRoute(app.httpServer.GetEngine(), app.clientMgr)
	
	client.SetGlobalClientManager(app.clientMgr)

	if app.config.FeatureIfaceTraffic {
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			timer.SetupInterfaceBroadcast(app.ctx)
		}()
	}

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		timer.UpdateSystemResource(app.ctx)
	}()

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.clientMgr.StartQueueHandler()
	}()

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.cleanupExpiredClients()
	}()

	return app.httpServer.Start()
}

func (app *Application) cleanupExpiredClients() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			removed := app.clientMgr.RemoveExpiredClients()
			if removed > 0 {
				log.Default().Printf("Cleaned up %d expired sessions\n", removed)
			}
		}
	}
}

func (app *Application) Shutdown() {
	log.Println("Application shutting down...")
	app.cancel()
	app.clientMgr.ShutdownQueue()
	app.wg.Wait()
	log.Println("Application shutdown complete")
}

func Init() {
	cfg := config.NewConfig()
	if err := cfg.LoadWebConfig(); err != nil {
		log.Printf("Warning: Failed to load web config: %v", err)
	}

	config.Config = cfg

	log.Default().Println("Listen on: " + cfg.ListenHost + ":" + cfg.ListenPort)

	app := NewApplication(cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Received shutdown signal")
		app.Shutdown()
	}()

	if err := app.Init(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}
