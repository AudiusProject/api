package main

import (
	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
)

func main() {
	as := api.NewApiServer(config.Cfg)
	as.Serve()
}
