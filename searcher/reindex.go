package searcher

import (
	"context"
	"log"
	"slices"

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
	esc, err := Dial(config.Cfg.EsUrl)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	return esc
}

func reindexPlaylists(base *BaseIndexer) {
	i := &PlaylistIndexer{base}

	if err := i.createIndex(); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}
}

func reindexUsers(base *BaseIndexer) {
	i := &UserIndexer{base}

	if err := i.createIndex(); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}
}

func reindexTracks(base *BaseIndexer) {
	i := &TrackIndexer{base}

	if err := i.createIndex(); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}
}

func reindexSocials(base *BaseIndexer) {
	i := &SocialIndexer{base}

	if err := i.createIndex(); err != nil {
		panic(err)
	}
	if err := i.indexAll(); err != nil {
		panic(err)
	}
}

func Reindex(pool *pgxpool.Pool, esc *elasticsearch.Client, drop bool, collections ...string) {

	baseIndexer := &BaseIndexer{
		pool,
		esc,
		drop,
	}

	reindexAll := len(collections) == 0

	if reindexAll || slices.Contains(collections, "playlists") {
		reindexPlaylists(baseIndexer)
	}
	if reindexAll || slices.Contains(collections, "tracks") {
		reindexTracks(baseIndexer)
	}
	if reindexAll || slices.Contains(collections, "users") {
		reindexUsers(baseIndexer)
	}
	if reindexAll || slices.Contains(collections, "socials") {
		reindexSocials(baseIndexer)
	}

}

func ReindexLegacy(drop bool, collections ...string) {
	pool := mustDialPostgres()
	esc := mustDialElasticsearch()
	Reindex(pool, esc, drop, collections...)
}
