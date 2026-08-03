package indexer

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	etl "github.com/OpenAudio/go-openaudio/pkg/etl"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// runPlaysHook drives the plays indexing logic for one Plays tx against a
// real DB. It calls indexPlays directly (not the hook wrapper) so a DB error
// surfaces as a test failure instead of being swallowed by the PlaysHook
// "log and continue" contract.
func runPlaysHook(t *testing.T, pool *pgxpool.Pool, height int64, plays []*corev1.TrackPlay) {
	t.Helper()
	err := indexPlays(context.Background(), &etl.PlaysParams{
		Plays:       plays,
		BlockHeight: height,
		BlockTime:   time.Unix(1700000000, 0),
		BlockHash:   "blockhash",
		TxHash:      "txhash",
		DBTX:        pool,
	})
	require.NoError(t, err)
}

func play(userID, trackID string) *corev1.TrackPlay {
	return &corev1.TrackPlay{
		UserId:    userID,
		TrackId:   trackID,
		Signature: "sig-" + userID + "-" + trackID,
		City:      "Brooklyn",
		Region:    "NY",
		Country:   "US",
		Timestamp: timestamppb.New(time.Unix(1700000000, 0)),
	}
}

func TestPlaysHook_InsertsPlay(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	runPlaysHook(t, pool, 555, []*corev1.TrackPlay{play("100", "200")})

	var (
		userID    *int64
		playItem  int64
		source    string
		slot      *int64
		signature *string
		city      *string
	)
	err := pool.QueryRow(context.Background(),
		`SELECT user_id, play_item_id, source, slot, signature, city
		 FROM plays WHERE play_item_id = 200`).
		Scan(&userID, &playItem, &source, &slot, &signature, &city)
	require.NoError(t, err)

	require.NotNil(t, userID)
	assert.Equal(t, int64(100), *userID)
	assert.Equal(t, int64(200), playItem)
	assert.Equal(t, "relay", source)
	require.NotNil(t, slot)
	assert.Equal(t, int64(555), *slot, "slot should be the Core block height")
	require.NotNil(t, signature)
	assert.Equal(t, "sig-100-200", *signature)
	require.NotNil(t, city)
	assert.Equal(t, "Brooklyn", *city)
}

// A non-integer user_id is an anonymous listen: the row is still written but
// user_id is NULL (mirrors index_core_plays).
func TestPlaysHook_AnonymousListen(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	runPlaysHook(t, pool, 1, []*corev1.TrackPlay{play("anon-device-uuid", "300")})

	var userID *int64
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*), max(user_id) FROM plays WHERE play_item_id = 300`).
		Scan(&count, &userID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "anonymous play is still recorded")
	assert.Nil(t, userID, "anonymous play has NULL user_id")
}

// A non-integer track_id is skipped entirely (Python's try/except continue).
func TestPlaysHook_SkipsNonIntegerTrackId(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	runPlaysHook(t, pool, 1, []*corev1.TrackPlay{
		play("100", "not-a-number"),
		play("100", "400"), // valid one in the same tx still lands
	})

	var total, valid int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM plays`).Scan(&total))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM plays WHERE play_item_id = 400`).Scan(&valid))
	assert.Equal(t, 1, total, "only the integer-track_id play is written")
	assert.Equal(t, 1, valid)
}

func TestPlaysHook_EmptyIsNoOp(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	runPlaysHook(t, pool, 1, nil)
	runPlaysHook(t, pool, 1, []*corev1.TrackPlay{})

	var total int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM plays`).Scan(&total))
	assert.Equal(t, 0, total)
}

// The on_play trigger fans a play out to aggregate_plays; verify the bridge
// row actually drives those downstream aggregates (the whole reason we write
// `plays` and not just `etl_plays`).
func TestPlaysHook_DrivesAggregatePlays(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	runPlaysHook(t, pool, 1, []*corev1.TrackPlay{
		play("100", "200"),
		play("101", "200"),
	})

	var count int64
	err := pool.QueryRow(context.Background(),
		`SELECT count FROM aggregate_plays WHERE play_item_id = 200`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "on_play trigger should have counted both plays")
}
