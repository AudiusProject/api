package jobs

import (
	"context"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
)

// How many tokens to fetch from the database in one batch.
const tokenPageSize = 1000

type ArtistCoin struct {
	Mint string  `db:"mint"`
	Pool *string `db:"dbc_pool"`
}

func GetTokenCount(ctx context.Context, pool database.DbPool) (int, error) {
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM artist_coins").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetTokenBatch(ctx context.Context, pool database.DbPool, limit, offset int) ([]ArtistCoin, error) {
	rows, err := pool.Query(ctx, "SELECT mint, dbc_pool FROM artist_coins ORDER BY mint LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}

	coins, err := pgx.CollectRows(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return nil, err
	}

	return coins, nil
}
