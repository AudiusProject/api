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
	"bridgerton.audius.co/ddl"
	"bridgerton.audius.co/esindexer"
	"bridgerton.audius.co/indexer"
	solana_indexer "bridgerton.audius.co/solana/indexer"
)

func main() {
	command := "server"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "server":
		{
			fmt.Println("Running migrations...")
			ddl.RunMigrations()

			fmt.Println("Running server...")
			as := api.NewApiServer(config.Cfg)
			as.Serve()
		}
	case "indexer":
		{
			fmt.Println("Running indexer...")
			ddl.RunMigrations()
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
			fmt.Println("Running solana-indexer...")
			solanaIndexer := solana_indexer.New(config.Cfg)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- solanaIndexer.Start(ctx)
			}()
			sigCh := make(chan os.Signal, 3)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			select {
			case <-sigCh:
				fmt.Println("Shutting down...")
				cancel()
				<-done
			case err := <-done:
				if err != nil {
					panic(err)
				}
				fmt.Println("Done.")
			}
		}
	default:
		fmt.Printf("Unrecognized command: %s", command)
	}
}
