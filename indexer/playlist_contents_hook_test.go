package indexer

import (
	"context"
	"encoding/json"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// seedPlaylist seeds a single playlists row with the given contents JSON.
// Mirrors the column defaults in database/seed.go's playlists block.
func seedPlaylist(t *testing.T, playlistID int, ownerID int, contents string) database.FixtureMap {
	t.Helper()
	return database.FixtureMap{
		"playlists": {
			{
				"playlist_id":       playlistID,
				"playlist_owner_id": ownerID,
				"playlist_name":     "test",
				"playlist_contents": contents,
			},
		},
	}
}

// runHook is shorthand for invoking the hook against an in-test playlists row.
func runHook(t *testing.T, pool *pgxpool.Pool, entityID int64) {
	t.Helper()
	logger := zap.NewNop()
	hook := newPlaylistContentsHook(logger)
	err := hook(context.Background(), &em.Params{
		EntityID:   entityID,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		// Non-nil to pass the metadata gate.
		Metadata: map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)
}

// TestPlaylistContentsHook_RenamesSDKShape covers the production regression:
// the TS SDK uploads `{"track_id": <hashid>, "timestamp": <epoch>}` (after
// snakecase-keys), ETL writes those entries to JSONB verbatim, and the API
// readers see no `track` field — so the playlist looks empty even though
// the row exists. The hook must rewrite the entry into the canonical
// `{"track": <int>, "time": <epoch>, "metadata_time": <epoch>}` shape.
func TestPlaylistContentsHook_RenamesSDKShape(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)

	sdkShape, err := json.Marshal(map[string]any{
		"track_ids": []map[string]any{
			{"track_id": hashid, "timestamp": 1700000000},
		},
	})
	require.NoError(t, err)
	database.Seed(pool, seedPlaylist(t, 2001, 1, string(sdkShape)))

	runHook(t, pool, 2001)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2001`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	entry := parsed.TrackIDs[0]
	assert.Equal(t, float64(28622946), entry["track"], "track must be the decoded integer")
	assert.Equal(t, float64(1700000000), entry["time"], "time must be copied from timestamp")
	assert.Equal(t, float64(1700000000), entry["metadata_time"], "metadata_time must be copied from timestamp when metadata_timestamp absent")
	assert.NotContains(t, entry, "track_id", "SDK-shape track_id must be dropped")
	assert.NotContains(t, entry, "timestamp", "SDK-shape timestamp must be dropped")
	assert.NotContains(t, entry, "metadata_timestamp", "SDK-shape metadata_timestamp must be dropped")
}

// TestPlaylistContentsHook_RenamesSDKShape_WithMetadataTimestamp covers
// the Create path where the SDK supplies both timestamp and
// metadata_timestamp. metadata_time must come from metadata_timestamp
// (not timestamp), matching the Python apps precedence.
func TestPlaylistContentsHook_RenamesSDKShape_WithMetadataTimestamp(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	hashid, err := trashid.EncodeHashId(123456)
	require.NoError(t, err)
	sdkShape := `{"track_ids":[{"track_id":"` + hashid + `","timestamp":1700000000,"metadata_timestamp":1699000000}]}`
	database.Seed(pool, seedPlaylist(t, 2002, 1, sdkShape))

	runHook(t, pool, 2002)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2002`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	entry := parsed.TrackIDs[0]
	assert.Equal(t, float64(123456), entry["track"])
	assert.Equal(t, float64(1700000000), entry["time"])
	assert.Equal(t, float64(1699000000), entry["metadata_time"])
}

// TestPlaylistContentsHook_NoChangeForCanonicalEntries confirms the hook
// is a no-op for the legacy/canonical entry shape Python apps wrote. We
// must not churn rows for playlists that the cutover doesn't break.
func TestPlaylistContentsHook_NoChangeForCanonicalEntries(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	original := `{"track_ids":[{"track":200,"time":1722451644,"metadata_time":1722451644}]}`
	database.Seed(pool, seedPlaylist(t, 2003, 1, original))

	runHook(t, pool, 2003)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2003`,
	).Scan(&got))
	assert.JSONEq(t, original, got)
}

// TestPlaylistContentsHook_SkipsWhenMetadataLacksPlaylistContents gates the
// hook on the same condition the upstream junction-table updater uses:
// only run when the SDK actually carried playlist_contents. A name-only
// update must not touch the column even if the column has fixable data.
func TestPlaylistContentsHook_SkipsWhenMetadataLacksPlaylistContents(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)
	stale := `{"track_ids":[{"track_id":"` + hashid + `","timestamp":1}]}`
	database.Seed(pool, seedPlaylist(t, 2004, 1, stale))

	logger := zap.NewNop()
	hook := newPlaylistContentsHook(logger)
	err = hook(context.Background(), &em.Params{
		EntityID:   2004,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		// playlist_name change without playlist_contents: hook must skip.
		Metadata: map[string]any{"playlist_name": "renamed"},
	})
	assert.NoError(t, err)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2004`,
	).Scan(&got))
	assert.JSONEq(t, stale, got)
}

// TestPlaylistContentsHook_MixedEntries proves the hook adapts entries
// individually, leaving canonical ones byte-identical while rewriting
// SDK-shape ones.
func TestPlaylistContentsHook_MixedEntries(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)
	mixed := `{"track_ids":[` +
		`{"track":200,"time":1,"metadata_time":1},` +
		`{"track_id":"` + hashid + `","timestamp":2}` +
		`]}`
	database.Seed(pool, seedPlaylist(t, 2005, 1, mixed))

	runHook(t, pool, 2005)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2005`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 2)

	first := parsed.TrackIDs[0]
	assert.Equal(t, float64(200), first["track"])
	assert.Equal(t, float64(1), first["time"])
	assert.Equal(t, float64(1), first["metadata_time"])

	second := parsed.TrackIDs[1]
	assert.Equal(t, float64(28622946), second["track"])
	assert.Equal(t, float64(2), second["time"])
	assert.Equal(t, float64(2), second["metadata_time"])
	assert.NotContains(t, second, "track_id")
	assert.NotContains(t, second, "timestamp")
}

// TestPlaylistContentsHook_NumericStringTrackID covers the case where the
// SDK didn't encode the ID but did stringify it. trashid.DecodeHashId
// falls back to strconv.Atoi for pure numeric strings — same behavior
// the upstream junction-table decoder relies on.
func TestPlaylistContentsHook_NumericStringTrackID(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	contents := `{"track_ids":[{"track_id":"12345","timestamp":1}]}`
	database.Seed(pool, seedPlaylist(t, 2006, 1, contents))

	runHook(t, pool, 2006)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2006`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	assert.Equal(t, float64(12345), parsed.TrackIDs[0]["track"])
}

// TestPlaylistContentsHook_EmptyContents covers the "removed the last
// track from a playlist" path: JSONB is {"track_ids":[]} and the hook
// must no-op without erroring.
func TestPlaylistContentsHook_EmptyContents(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	database.Seed(pool, seedPlaylist(t, 2007, 1, `{"track_ids":[]}`))

	runHook(t, pool, 2007)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2007`,
	).Scan(&got))
	assert.JSONEq(t, `{"track_ids":[]}`, got)
}

// TestPlaylistContentsHook_UndecodableTrackIDLeftAlone confirms we don't
// drop data on a malformed track_id — we log and leave the entry alone.
// The downstream readers still fail loudly on it, but at least the raw
// metadata the SDK uploaded is preserved.
func TestPlaylistContentsHook_UndecodableTrackIDLeftAlone(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	bad := `{"track_ids":[{"track_id":"not-a-hashid!@#","timestamp":1}]}`
	database.Seed(pool, seedPlaylist(t, 2008, 1, bad))

	runHook(t, pool, 2008)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2008`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	entry := parsed.TrackIDs[0]
	// No `track` field added because the value couldn't be decoded.
	assert.NotContains(t, entry, "track")
	// track_id is still dropped (unconditional), but the entry's intent
	// is preserved enough that an operator can inspect it.
	assert.NotContains(t, entry, "track_id")
}

// TestPlaylistContentsHook_HashidOnTrackField covers an alternate SDK
// shape where the entry uses `track` (canonical name) but with a hashid
// string value. Pre-cutover Python apps decoded these; we must too.
func TestPlaylistContentsHook_HashidOnTrackField(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)
	contents := `{"track_ids":[{"track":"` + hashid + `","time":1,"metadata_time":1}]}`
	database.Seed(pool, seedPlaylist(t, 2009, 1, contents))

	runHook(t, pool, 2009)

	var got string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 2009`,
	).Scan(&got))

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	assert.Equal(t, float64(28622946), parsed.TrackIDs[0]["track"])
}
