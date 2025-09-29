package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"api.audius.co/api"
	"api.audius.co/config"
	"api.audius.co/ddl"
	"api.audius.co/esindexer"
	"api.audius.co/indexer"
	solana_indexer "api.audius.co/solana/indexer"
)

func main() {
	command := "server"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "server":
		{
			ddl.RunMigrations(false)

			fmt.Println("Running server...")
			as := api.NewApiServer(config.Cfg)
			as.Serve()
		}
	case "indexer":
		{
			ddl.RunMigrations(false)
			fmt.Println("Running indexer...")
			_, err := indexer.NewIndexer(indexer.CoreIndexerConfig{
				DbUrl: config.Cfg.WriteDbUrl,
			})
			if err != nil {
				fmt.Println("Error creating indexer:", err)
				os.Exit(1)
			}
		}
	case "es-indexer":
		{
			collections := os.Args[2:]
			drop := slices.Contains(collections, "drop")
			fmt.Printf("Reindexing ElasticSearch (collections=%s, drop=%t)...\n", collections, drop)
			esindexer.ReindexLegacy(drop, collections...)
		}
	case "solana-indexer":
		{
			ddl.RunMigrations(false)
			fmt.Println("Running solana-indexer...")
			solanaIndexer := solana_indexer.New(config.Cfg)
			defer solanaIndexer.Close()

			// Capture termination signals for graceful shutdown of the indexer
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
			defer stop()

			if err := solanaIndexer.Start(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
			}
		}
	case "migrate":
		{
			ddl.RunMigrations(true)
			os.Exit(0)
		}
	default:
		fmt.Printf("Unrecognized command: %s", command)
	}
}
