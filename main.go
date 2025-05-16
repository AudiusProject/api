package main

import (
	"fmt"
	"os"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/searcher"
)

func main() {

	if len(os.Args) > 1 && os.Args[1] == "search" {
		fmt.Println("search time...")
		searcher.Demo()
		return
	}

	as := api.NewApiServer(config.Cfg)
	as.Serve()

}
