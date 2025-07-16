package esindexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"bridgerton.audius.co/config"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

func mustDialPostgres() *pgxpool.Pool {
	connConfig, err := pgxpool.ParseConfig(config.Cfg.WriteDbUrl)
	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(err)
	}
	return pool
}

func mustDialElasticsearch() *elasticsearch.Client {
	esc, err := Dial(config.Cfg.EsUrl)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	return esc
}

func Reindex(pool *pgxpool.Pool, esc *elasticsearch.Client, drop bool, collections ...string) {

	bulk, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:     esc,
		NumWorkers: 2,
		Refresh:    "true",
	})
	if err != nil {
		log.Fatalf("Error creating the indexer: %s", err)
	}

	esIndexer := &EsIndexer{
		pool,
		esc,
		bulk,
		drop,
	}

	// this is just a "listen" demo for now...
	// this will block forever
	// todo: need to figure out if this runs after any re-index, or concurrently...
	if slices.Contains(collections, "listen") {
		ctx, cancel := context.WithCancel(context.Background())

		// Listen for Ctrl+C (SIGINT) and call cancel when received
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("Listen exit")
			cancel()
			os.Exit(0)
		}()

		esIndexer.listen(ctx)

	}

	reindexAll := len(collections) == 0 || slices.Contains(collections, "all")

	g := errgroup.Group{}

	if reindexAll || slices.Contains(collections, "playlists") {
		g.Go(func() error {
			return esIndexer.reindexCollection("playlists")
		})
	}
	if reindexAll || slices.Contains(collections, "tracks") {
		g.Go(func() error {
			return esIndexer.reindexCollection("tracks")
		})
	}
	if reindexAll || slices.Contains(collections, "users") {
		g.Go(func() error {
			return esIndexer.reindexCollection("users")
		})
	}
	if reindexAll || slices.Contains(collections, "socials") {
		g.Go(func() error {
			return esIndexer.reindexCollection("socials")
		})
	}

	err = g.Wait()
	if err != nil {
		panic(err)
	}

	esIndexer.bulk.Close(context.Background())

}

func ReindexForTest(pool *pgxpool.Pool, esc *elasticsearch.Client) {

	bulk, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:     esc,
		NumWorkers: 2,
		Refresh:    "true",
	})
	if err != nil {
		log.Fatalf("Error creating the indexer: %s", err)
	}

	esIndexer := &EsIndexer{
		pool,
		esc,
		bulk,
		true,
	}

	err = esIndexer.reindexAll()
	if err != nil {
		panic(err)
	}

	esIndexer.bulk.Close(context.Background())
}

func ReindexLegacy(drop bool, collections ...string) {
	pool := mustDialPostgres()
	esc := mustDialElasticsearch()
	Reindex(pool, esc, drop, collections...)
}
