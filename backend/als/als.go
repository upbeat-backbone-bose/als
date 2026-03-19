package als

import (
	"log"
	"time"

	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/timer"
	"github.com/samlm0/als/v2/config"
	alsHttp "github.com/samlm0/als/v2/http"
)

func Init() {
	aHttp := alsHttp.CreateServer()

	log.Default().Println("Listen on: " + config.Config.ListenHost + ":" + config.Config.ListenPort)
	aHttp.SetListen(config.Config.ListenHost + ":" + config.Config.ListenPort)

	SetupHttpRoute(aHttp.GetEngine())

	if config.Config.FeatureIfaceTraffic {
		go timer.SetupInterfaceBroadcast()
	}
	go timer.UpdateSystemResource()
	go client.HandleQueue()
	go cleanupExpiredClients()
	aHttp.Start()
}

func cleanupExpiredClients() {
	ticker := time.NewTicker(1 * time.Hour)
	for {
		<-ticker.C
		removed := client.RemoveExpiredClients()
		if removed > 0 {
			log.Default().Printf("Cleaned up %d expired sessions\n", removed)
		}
	}
}
