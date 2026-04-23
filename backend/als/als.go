package als

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
	alsHttp "github.com/samlm0/als/v2/http"
)

type Application struct {
	config      *config.ALSConfig
	clientMgr   *client.ClientManager
	queueMgr    *client.QueueManager
	httpServer  *alsHttp.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewApplication(cfg *config.ALSConfig) *Application {
	ctx, cancel := context.WithCancel(context.Background())
	return &Application{
		config:     cfg,
		clientMgr:  client.NewClientManager(),
		queueMgr:   client.NewQueueManager(),
		httpServer: alsHttp.CreateServer(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (app *Application) Init() error {
	app.httpServer.SetListen(app.config.ListenHost + ":" + app.config.ListenPort)

	SetupHttpRoute(app.httpServer.GetEngine(), app.clientMgr)

	if app.config.FeatureIfaceTraffic {
		go timer.SetupInterfaceBroadcast()
	}
	go timer.UpdateSystemResource()
	go app.queueMgr.HandleQueue()
	go app.handleQueueWrapper()
	go app.cleanupExpiredClients()

	return app.httpServer.Start()
}

func (app *Application) handleQueueWrapper() {
	// Wrapper to inject clientMgr context
	for {
		select {
		case <-app.ctx.Done():
			return
		default:
			// ClientManager handles its own context
		}
	}
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
	app.queueMgr.Shutdown()
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
