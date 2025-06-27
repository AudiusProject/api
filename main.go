package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/esindexer"
	"bridgerton.audius.co/solana/indexers"
)

func main() {
	command := "server"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "es-indexer":
		{

			collections := os.Args[2:]
			drop := slices.Contains(collections, "drop")
			fmt.Printf("Reindexing ElasticSearch (collections=%s, drop=%t)...\n", collections, drop)
			esindexer.ReindexLegacy(drop, collections...)
		}
	case "solana-indexer":
		{
			sigCh := make(chan os.Signal, 1)
			fmt.Println("Running indexer...")
			tokenIndexer := indexers.NewSolanaIndexer(config.Cfg)
			go tokenIndexer.Start(context.Background())
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
		}
	case "server":
		{
			fmt.Println("Running server...")
			as := api.NewApiServer(config.Cfg)
			as.Serve()
		}
	default:
		fmt.Printf("Unrecognized command: %s", command)
	}
}
