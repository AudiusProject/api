package main

import (
	"fmt"
	"os"
	"slices"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/searcher"
)

func main() {

	if len(os.Args) > 1 && os.Args[1] == "reindex" {
		collections := os.Args[2:]
		drop := slices.Contains(collections, "drop")
		fmt.Println("reindex", "drop", drop, "collections", collections)

		searcher.ReindexLegacy(drop, collections...)
		return
	}

	as := api.NewApiServer(config.Cfg)
	as.Serve()

}
