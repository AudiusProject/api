package main

import (
	"context"
	"flag"
	"fmt"
	"sync"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/solana/indexers"
)

func main() {
	enableServer := flag.Bool("server", true, "Enable the webserver")
	enableIndexer := flag.Bool("indexer", false, "Enable the indexer")
	flag.Parse()

	var wg sync.WaitGroup

	if enableIndexer != nil && *enableIndexer {
		fmt.Println("Running indexer...")
		wg.Add(1)
		tokenIndexer := indexers.NewSolanaIndexer(config.Cfg)
		go tokenIndexer.Start(context.Background())
	}

	if enableServer == nil || *enableServer {
		fmt.Println("Running server...")
		wg.Add(1)
		as := api.NewApiServer(config.Cfg)
		go as.Serve()
	}

	wg.Wait() // Never finishes
}
