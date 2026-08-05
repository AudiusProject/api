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
	eth_indexer "api.audius.co/eth/indexer"
	core_indexer "api.audius.co/indexer"
	"api.audius.co/logging"
	solana_indexer "api.audius.co/solana/indexer"
	etldb "github.com/OpenAudio/go-openaudio/pkg/etl/db"
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
		if err := ddl.RunMigrations(); err != nil {
			fmt.Println("migration failed:", err)
			os.Exit(1)
		}
	}

	switch command {
	case "server":
		{
			fmt.Println("Running server...")
			as := api.NewApiServer(config.Cfg)
			as.Serve()
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

			healthServer := solana_indexer.NewServer(solanaIndexer)

			// Capture termination signals for graceful shutdown of the indexer
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
			defer stop()

			go healthServer.Start(ctx)

			if err := solanaIndexer.Start(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
			}
		}
	case "eth-indexer":
		{
			fmt.Println("Running eth-indexer...")
			ethIndexer := eth_indexer.New(config.Cfg)
			defer ethIndexer.Close()

			healthServer := eth_indexer.NewServer(ethIndexer)

			// Capture termination signals for graceful shutdown of the indexer
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
			defer stop()

			go healthServer.Start(ctx)

			if err := ethIndexer.Start(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
			}
		}
	case "migrate":
		{
			// ddl migrations already ran above. Apply the ETL module's too, so a
			// database migrated by this command matches a deployed one.
			//
			// They are otherwise only ever applied at indexer start, which left
			// `make test-schema` unable to produce them: that target seeds from
			// sql/01_schema.sql, runs this command, and dumps the result, so
			// ETL-created objects could never enter the loop no matter how often
			// it was regenerated. The four partial unique indexes from pkg/etl
			// 0030 were consequently absent under test while present in every
			// deployment — which is how ddl 0236 came to be written against the
			// wrong constraint and still passed CI.
			//
			// Ordering is the useful side effect: ddl migrations run before these,
			// in one process, so a ddl migration that has to precede an ETL one
			// (0237 before 0035) is sequenced here rather than across two pods.
			//
			// Idempotent and tracked separately in etl_db_migrations, so the
			// indexer applying them again at startup is a no-op.
			logger := logging.NewZapLogger(config.Cfg).Named("migrate")
			if err := etldb.RunMigrations(logger, config.Cfg.WriteDbUrl, false); err != nil {
				fmt.Println("etl migration failed:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	default:
		fmt.Printf("Unrecognized command: %s", command)
	}
}
