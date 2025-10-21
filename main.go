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
	core_indexer "api.audius.co/indexer"
	solana_indexer "api.audius.co/solana/indexer"
)

func main() {
	command := "server"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if !config.Cfg.RunMigrations && command != "migrate" {
		fmt.Println("Skipping migrations. Set env runMigrations=true to run.")
	} else {
		fmt.Println("Running migrations...")
		ddl.RunMigrations()
	}

	switch command {
	case "server":
		{
			fmt.Println("Running server...")
			as := api.NewApiServer(config.Cfg)
			as.Serve()
		}
	case "aggregates-indexer":
		{
			fmt.Println("Running aggregates-indexer...")

			aggregatesIndexer := core_indexer.NewAggregatesIndexer(config.Cfg)

			defer aggregatesIndexer.Close()

			// Capture termination signals for graceful shutdown of the indexer
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
			defer stop()

			if err := aggregatesIndexer.Start(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
			}
		}
	case "indexer":
		{
			fmt.Println("Running indexer...")

			indexer := core_indexer.NewIndexer(config.Cfg)

			defer indexer.Close()

			// Capture termination signals for graceful shutdown of the indexer
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
			defer stop()

			if err := indexer.Start(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
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
			// no-op, handled prior to switch/case
			os.Exit(0)
		}
	default:
		fmt.Printf("Unrecognized command: %s", command)
	}
}
