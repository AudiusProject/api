package indexer

import (
	"context"
	"testing"

	"api.audius.co/database"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexEvents drives the user_events indexing logic for a single User
// Create/Update tx whose metadata carries the given `events` sub-object.
// It calls indexUserEvents directly (rather than the hook wrapper) so a
// real DB error surfaces as a test failure instead of being swallowed by
// the PostHook "log and continue" contract.
func indexEvents(t *testing.T, pool *pgxpool.Pool, userID int64, events map[string]any) {
	t.Helper()
	err := indexUserEvents(context.Background(), &em.Params{
		EntityID:    userID,
		Metadata:    map[string]any{"events": events},
		BlockNumber: 101, // matches the block database.Seed always inserts
		BlockHash:   "block1",
		DBTX:        pool,
	})
	require.NoError(t, err)
}

// currentUserEvent returns the single is_current user_events row for a user.
func currentUserEvent(t *testing.T, pool *pgxpool.Pool, userID int64) (mobile bool, referrer *int64, ok bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT is_mobile_user, referrer FROM user_events WHERE user_id = $1 AND is_current = true`,
		userID).Scan(&mobile, &referrer)
	if err == pgx.ErrNoRows {
		return false, nil, false
	}
	require.NoError(t, err)
	return mobile, referrer, true
}

// userEventRowCount counts all (historical + current) rows for a user.
func userEventRowCount(t *testing.T, pool *pgxpool.Pool, userID int64) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM user_events WHERE user_id = $1`, userID).Scan(&n))
	return n
}

func TestUserEventsHook_MobileInsert(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	indexEvents(t, pool, 700, map[string]any{"is_mobile_user": true})

	mobile, referrer, ok := currentUserEvent(t, pool, 700)
	require.True(t, ok, "expected a current user_events row")
	assert.True(t, mobile)
	assert.Nil(t, referrer)
}

func TestUserEventsHook_ReferrerInsert(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	// JSON numbers decode to float64 — exercise that path.
	indexEvents(t, pool, 600, map[string]any{"referrer": float64(500)})

	mobile, referrer, ok := currentUserEvent(t, pool, 600)
	require.True(t, ok)
	assert.False(t, mobile)
	if assert.NotNil(t, referrer) {
		assert.Equal(t, int64(500), *referrer)
	}
}

// is_mobile_user is sticky and referrer is set-once: a later tx that adds a
// referrer must keep mobile=true, and a still-later tx must not overwrite an
// already-set referrer. Each merge versions the row via is_current.
func TestUserEventsHook_StickyMobileAndSetOnceReferrer(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	indexEvents(t, pool, 700, map[string]any{"is_mobile_user": true})
	indexEvents(t, pool, 700, map[string]any{"referrer": float64(500)})

	mobile, referrer, ok := currentUserEvent(t, pool, 700)
	require.True(t, ok)
	assert.True(t, mobile, "mobile must stay true after the referrer tx")
	if assert.NotNil(t, referrer) {
		assert.Equal(t, int64(500), *referrer)
	}
	assert.Equal(t, 2, userEventRowCount(t, pool, 700), "prior row demoted, not deleted")

	// A second referrer must not overwrite the first (set-once) — no new row.
	indexEvents(t, pool, 700, map[string]any{"referrer": float64(999)})
	_, referrer, _ = currentUserEvent(t, pool, 700)
	if assert.NotNil(t, referrer) {
		assert.Equal(t, int64(500), *referrer, "referrer is set-once")
	}
	assert.Equal(t, 2, userEventRowCount(t, pool, 700), "no-op must not version a new row")
}

func TestUserEventsHook_NoOpWhenUnchanged(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	indexEvents(t, pool, 700, map[string]any{"is_mobile_user": true})
	indexEvents(t, pool, 700, map[string]any{"is_mobile_user": true}) // identical re-send

	assert.Equal(t, 1, userEventRowCount(t, pool, 700), "repeat of same events is a no-op")
}

func TestUserEventsHook_SelfReferralIgnored(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	// referrer == self and no mobile flag → nothing relevant, no row written.
	indexEvents(t, pool, 700, map[string]any{"referrer": float64(700)})

	_, _, ok := currentUserEvent(t, pool, 700)
	assert.False(t, ok)
	assert.Equal(t, 0, userEventRowCount(t, pool, 700))
}

func TestUserEventsHook_NoEventsObject(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, database.FixtureMap{})

	// Metadata with no `events` sub-object → hook is a no-op.
	err := indexUserEvents(context.Background(), &em.Params{
		EntityID:    700,
		Metadata:    map[string]any{"name": "ray"},
		BlockNumber: 101,
		BlockHash:   "block1",
		DBTX:        pool,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, userEventRowCount(t, pool, 700))
}
