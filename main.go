package main

import (
	"fmt"
	"os"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/searcher"
)

func main() {

	if len(os.Args) > 1 && os.Args[1] == "reindex" {
		collections := os.Args[2:]
		fmt.Println("reindex", collections)

		searcher.ReindexLegacy(collections...)
		return
	}

	as := api.NewApiServer(config.Cfg)
	as.Serve()

}
