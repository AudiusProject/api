package common

import (
	"context"
	"fmt"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
)

func GetArtistCoinMints(ctx context.Context, db database.DBTX, limit int, offset int) ([]string, error) {
	sqlMints := `SELECT mint FROM artist_coins LIMIT @limit OFFSET @offset`
	rows, err := db.Query(ctx, sqlMints, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})
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
	return mintAddresses, nil
}
