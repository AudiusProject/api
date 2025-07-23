package indexer

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/database"
	"github.com/jackc/pgx/v5"
)

func withRetries[T any](f func() (T, error), maxRetries int, interval time.Duration) (T, error) {
	result, err := f()
	retries := 0
	for err != nil && retries < maxRetries {
		time.Sleep(interval)
		result, err = f()
		retries++
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("retry failed: %w", err)
	}
	return result, nil
}

var mintsCache []string

func getArtistCoins(ctx context.Context, db database.DBTX, forceRefresh bool) ([]string, error) {
	if !forceRefresh && mintsCache != nil {
		return mintsCache, nil
	}
	sqlMints := `SELECT mint FROM artist_coins`
	rows, err := db.Query(ctx, sqlMints)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No mints found, return empty slice
		}
		return nil, fmt.Errorf("failed to query mints: %w", err)
	}
	mintAddresses, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("failed to collect mints: %w", err)
	}
	mintsCache = mintAddresses
	return mintAddresses, nil
}
