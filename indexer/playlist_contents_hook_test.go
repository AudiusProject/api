package indexer

import (
	"context"
	"encoding/json"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// seedPlaylist seeds a single playlists row with the given contents JSON.
// Mirrors the column defaults wired into database/seed.go's playlists block.
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

// TestPlaylistContentsHook_DecodesHashidTrackToInt is the core regression
// for the apps#... cutover bug: the SDK uploads a hashid `track` field,
// ETL's normalizePlaylistContentsJSON passes it verbatim, and downstream
// readers can't cast the string to int. The hook must rewrite the column
// with the decoded integer.
func TestPlaylistContentsHook_DecodesHashidTrackToInt(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)

	contents, err := json.Marshal(map[string]any{
		"track_ids": []map[string]any{
			{"track": hashid, "time": 1722451644, "metadata_time": 1722451644},
		},
	})
	require.NoError(t, err)
	database.Seed(pool, seedPlaylist(t, 1001, 1, string(contents)))

	hook := newPlaylistContentsHook(logger)
	err = hook(context.Background(), &em.Params{
		EntityID:   1001,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		// non-nil so the metadata gate passes
		Metadata: map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1001`,
	).Scan(&got)
	require.NoError(t, err)

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	// json.Unmarshal into any decodes JSON numbers as float64
	assert.Equal(t, float64(28622946), parsed.TrackIDs[0]["track"])
}

// TestPlaylistContentsHook_NoChangeForIntegerTracks confirms the hook is
// a no-op for the common case where the SDK already supplied an integer
// track ID. The JSONB must remain byte-identical so we don't churn rows
// for playlists that aren't affected by the bug.
func TestPlaylistContentsHook_NoChangeForIntegerTracks(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	original := `{"track_ids":[{"track":200,"time":1722451644,"metadata_time":1722451644}]}`
	database.Seed(pool, seedPlaylist(t, 1002, 1, original))

	hook := newPlaylistContentsHook(logger)
	err := hook(context.Background(), &em.Params{
		EntityID:   1002,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		Metadata:   map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1002`,
	).Scan(&got)
	require.NoError(t, err)
	assert.JSONEq(t, original, got)
}

// TestPlaylistContentsHook_SkipsWhenMetadataLacksPlaylistContents gates the
// hook on the same condition the upstream junction-table updater uses:
// only run when the SDK actually carried playlist_contents in the tx. A
// name-only update must not touch the column.
func TestPlaylistContentsHook_SkipsWhenMetadataLacksPlaylistContents(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)
	stale := `{"track_ids":[{"track":"` + hashid + `","time":1}]}`
	database.Seed(pool, seedPlaylist(t, 1003, 1, stale))

	hook := newPlaylistContentsHook(logger)
	err = hook(context.Background(), &em.Params{
		EntityID:   1003,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		// playlist_name change without playlist_contents — the hook must
		// not touch the JSONB, even though it contains a fixable string.
		Metadata: map[string]any{"playlist_name": "renamed"},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1003`,
	).Scan(&got)
	require.NoError(t, err)
	assert.JSONEq(t, stale, got)
}

// TestPlaylistContentsHook_MixedEntries proves the hook decodes only the
// entries that need it, leaving integer-form entries byte-identical and
// only rewriting the misformatted ones.
func TestPlaylistContentsHook_MixedEntries(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	hashid, err := trashid.EncodeHashId(28622946)
	require.NoError(t, err)
	mixed := `{"track_ids":[{"track":200,"time":1},{"track":"` + hashid + `","time":2}]}`
	database.Seed(pool, seedPlaylist(t, 1004, 1, mixed))

	hook := newPlaylistContentsHook(logger)
	err = hook(context.Background(), &em.Params{
		EntityID:   1004,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		Metadata:   map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1004`,
	).Scan(&got)
	require.NoError(t, err)

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 2)
	assert.Equal(t, float64(200), parsed.TrackIDs[0]["track"])
	assert.Equal(t, float64(28622946), parsed.TrackIDs[1]["track"])
}

// TestPlaylistContentsHook_NumericStringDecoded covers the trashid
// behavior the junction-table decoder also has: a "12345"-style numeric
// string falls back to strconv.Atoi rather than the Hashids algorithm.
// Including this case is important because the SDK has historically
// stringified IDs in JSON without hashid-encoding them.
func TestPlaylistContentsHook_NumericStringDecoded(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	contents := `{"track_ids":[{"track":"12345","time":1}]}`
	database.Seed(pool, seedPlaylist(t, 1005, 1, contents))

	hook := newPlaylistContentsHook(logger)
	err := hook(context.Background(), &em.Params{
		EntityID:   1005,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		Metadata:   map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1005`,
	).Scan(&got)
	require.NoError(t, err)

	var parsed struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	require.Len(t, parsed.TrackIDs, 1)
	assert.Equal(t, float64(12345), parsed.TrackIDs[0]["track"])
}

// TestPlaylistContentsHook_EmptyContents covers the common "removed the
// last track from a playlist" path. The JSONB is {"track_ids":[]}; the
// hook must no-op without erroring.
func TestPlaylistContentsHook_EmptyContents(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	database.Seed(pool, seedPlaylist(t, 1006, 1, `{"track_ids":[]}`))

	hook := newPlaylistContentsHook(logger)
	err := hook(context.Background(), &em.Params{
		EntityID:   1006,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		Metadata:   map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1006`,
	).Scan(&got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"track_ids":[]}`, got)
}

// TestPlaylistContentsHook_UndecodableStringLeftAsIs proves we don't
// silently drop entries whose `track` value isn't a valid hashid — we
// log and continue, leaving the entry untouched. Dropping would be worse
// than the current behavior: downstream readers still fail loudly, but
// the metadata as the SDK uploaded it is preserved.
func TestPlaylistContentsHook_UndecodableStringLeftAsIs(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	bad := `{"track_ids":[{"track":"not-a-hashid!@#","time":1}]}`
	database.Seed(pool, seedPlaylist(t, 1007, 1, bad))

	hook := newPlaylistContentsHook(logger)
	err := hook(context.Background(), &em.Params{
		EntityID:   1007,
		EntityType: em.EntityTypePlaylist,
		Action:     em.ActionUpdate,
		DBTX:       pool,
		Metadata:   map[string]any{"playlist_contents": []any{}},
	})
	assert.NoError(t, err)

	var got string
	err = pool.QueryRow(context.Background(),
		`SELECT playlist_contents::text FROM playlists WHERE playlist_id = 1007`,
	).Scan(&got)
	require.NoError(t, err)
	assert.JSONEq(t, bad, got)
}
