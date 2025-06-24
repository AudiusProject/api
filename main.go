package main

import (
	"context"

	"bridgerton.audius.co/api"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/solana/indexers"
)

func main() {
	tokenIndexer := indexers.NewSolanaIndexer(config.Cfg)
	go tokenIndexer.Start(context.Background())

	as := api.NewApiServer(config.Cfg)
	as.Serve()
}
