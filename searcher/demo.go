package searcher

import (
	"context"
	"log"

	"bridgerton.audius.co/config"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustDialPostgres() *pgxpool.Pool {
	connConfig, err := pgxpool.ParseConfig(config.Cfg.DbUrl)
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
	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://35.238.44.255:21302",
		},
	}

	esc, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	return esc
}

func Demo() {
	pool := mustDialPostgres()
	esc := mustDialElasticsearch()

	baseIndexer := &BaseIndexer{
		pool,
		esc,
	}

	userIndexer := &UserIndexer{baseIndexer}

	if err := userIndexer.createIndex(true); err != nil {
		panic(err)
	}
	if err := userIndexer.indexAll(); err != nil {
		panic(err)
	}

	userIndexer.search("ray")

	trackIndexer := &TrackIndexer{baseIndexer}

	if err := trackIndexer.createIndex(true); err != nil {
		panic(err)
	}
	if err := trackIndexer.indexAll(); err != nil {
		panic(err)
	}

	trackIndexer.search("rap")
}
