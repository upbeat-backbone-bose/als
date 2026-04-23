package main

import (
	"flag"

	"github.com/samlm0/als/v2/als"
	"github.com/samlm0/als/v2/config"
	"github.com/samlm0/als/v2/fakeshell"
)

var shell = flag.Bool("shell", false, "Start as fake shell")

func main() {
	flag.Parse()
	if *shell {
		config.IsInternalCall = true
		cfg := config.NewConfig()
		cfg.Load()
		config.Config = cfg
		fakeshell.HandleConsole()
		return
	}

	als.Init()
}
