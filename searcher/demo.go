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
	esc, err := Dial()
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	return esc
}

func playlistDemo(base *BaseIndexer) {
	i := &PlaylistIndexer{base}

	if err := i.createIndex(true); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}

	i.search("ray")
}

func userDemo(base *BaseIndexer) {
	i := &UserIndexer{base}

	if err := i.createIndex(true); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}

	i.search("ray")
}

func trackDemo(base *BaseIndexer) {
	i := &TrackIndexer{base}

	if err := i.createIndex(true); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}

	i.search("rap")
}

func Demo() {
	pool := mustDialPostgres()
	esc := mustDialElasticsearch()

	baseIndexer := &BaseIndexer{
		pool,
		esc,
	}

	// playlistDemo(baseIndexer)
	trackDemo(baseIndexer)
}
