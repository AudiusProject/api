package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"bridgerton.audius.co/queries"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	_ "github.com/joho/godotenv/autoload"
)

func TestDB(t *testing.T) {
	ctx := t.Context()

	dbUrl := "postgres://postgres:example@localhost:21300/postgres"
	conn, err := pgx.Connect(ctx, dbUrl)
	assert.NoError(t, err)
	defer conn.Close(ctx)

	var name string
	var weight int64
	err = conn.QueryRow(ctx, "select 'bridge', 2").Scan(&name, &weight)
	assert.NoError(t, err)

	assert.Equal(t, "bridge", name)
	assert.EqualValues(t, 2, weight)
	fmt.Println(name, weight)

	tx, err := conn.Begin(ctx)
	assert.NoError(t, err)

	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
	INSERT INTO public.users (
		user_id,
		handle,
		is_current,
		is_verified,
		created_at,
		updated_at,
		has_collectibles,
		txhash,
		is_deactivated,
		is_available,
		is_storage_v2,
		allow_ai_attribution
	) VALUES (
		1,                    -- user_id
		'testing',            -- handle
		true,                 -- is_current
		false,                -- is_verified
		CURRENT_TIMESTAMP,    -- created_at
		CURRENT_TIMESTAMP,    -- updated_at
		false,                -- has_collectibles
		'',                   -- txhash
		false,                -- is_deactivated
		true,                 -- is_available
		false,                -- is_storage_v2
		false                 -- allow_ai_attribution
	);`)

	_, err = tx.Exec(ctx, `
	INSERT INTO public.users (
		user_id,
		handle,
		is_current,
		is_verified,
		created_at,
		updated_at,
		has_collectibles,
		txhash,
		is_deactivated,
		is_available,
		is_storage_v2,
		allow_ai_attribution
	) VALUES (
		2,                    -- user_id
		'dave',               -- handle
		true,                 -- is_current
		false,                -- is_verified
		CURRENT_TIMESTAMP,    -- created_at
		CURRENT_TIMESTAMP,    -- updated_at
		false,                -- has_collectibles
		'',                   -- txhash
		false,                -- is_deactivated
		true,                 -- is_available
		false,                -- is_storage_v2
		false                 -- allow_ai_attribution
	);`)
	assert.NoError(t, err)

	_, err = tx.Exec(ctx, `
	INSERT INTO public.blocks (
		blockhash,
		parenthash,
		is_current,
		number
	) VALUES (
		'block_abc123',   -- blockhash
		'block_def456',   -- parenthash
		true,             -- is_current
		101               -- number
	);
	`)
	assert.NoError(t, err)

	_, err = tx.Exec(ctx, `
	INSERT INTO public.follows (
		blockhash,
		blocknumber,
		follower_user_id,
		followee_user_id,
		is_current,
		is_delete,
		created_at,
		txhash,
		slot
	) VALUES (
		'abc123',         -- blockhash
		101,              -- blocknumber
		1,                -- follower_user_id
		2,                -- followee_user_id
		true,             -- is_current
		false,            -- is_delete
		CURRENT_TIMESTAMP,-- created_at
		'tx123',          -- txhash
		500               -- slot
	);
	`)
	assert.NoError(t, err)

	{
		var handle string
		err = tx.QueryRow(ctx, "select handle from users where user_id = $1", 1).Scan(&handle)
		assert.NoError(t, err)
		assert.Equal(t, "testing", handle)

		// trigger creates + updates aggregate_user row
		trackCount := -1
		followingCount := -1
		err = tx.QueryRow(ctx, "select track_count, following_count from aggregate_user where user_id = $1", 1).Scan(&trackCount, &followingCount)
		assert.NoError(t, err)
		assert.Equal(t, 0, trackCount)
		assert.Equal(t, 1, followingCount)

		err = tx.QueryRow(ctx, "select track_count, following_count from aggregate_user where user_id = $1", 2).Scan(&trackCount, &followingCount)
		assert.NoError(t, err)
		assert.Equal(t, 0, trackCount)
	}
}

func TestProdDB(t *testing.T) {
	t.Skip()
	ctx := context.Background()
	dbUrl := os.Getenv("discoveryDbUrl")
	conn, err := pgx.Connect(ctx, dbUrl)
	assert.NoError(t, err)
	defer conn.Close(ctx)

	var handle string
	err = conn.QueryRow(ctx, "select handle from users where user_id = $1", 1).Scan(&handle)
	assert.NoError(t, err)
	assert.Equal(t, "rayjacobson", handle)

	// use sqlc
	q := queries.New(conn)
	h := "stereosteve"
	user, err := q.GetUserByHandle(ctx, h)
	assert.NoError(t, err)
	assert.Equal(t, *user.Wallet, "0x613d83f44970ead52afc256b4e81766304f1d0fc")

	u, err := json.MarshalIndent(user, "", "  ")
	assert.NoError(t, err)
	fmt.Println(string(u))

}
