package indexer

import (
	"context"
	"encoding/json"

	"api.audius.co/trashid"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"go.uber.org/zap"
)

// newPlaylistContentsHook returns a pkg/etl post-hook that decodes any
// hashid-encoded track IDs in playlists.playlist_contents back to integers,
// so the JSONB stored in the column matches what API readers expect.
//
// Background: ETL's playlist Create/Update handlers split track-ID handling
// into two paths:
//
//   - playlist_tracks junction: built via pickPlaylistTrackID, which
//     hashid-decodes any string `track` value before insert.
//   - playlists.playlist_contents JSONB: written through
//     normalizePlaylistContentsJSON, which json.Marshals entries verbatim
//     and does NOT decode strings.
//
// If the SDK uploads a hashid-encoded `track` field (e.g. "1aV5byE"
// instead of 28622946), the JSONB ends up holding the string, while the
// downstream readers both expect an integer:
//
//   - api/v1_playlist_tracks.go casts (e.value->>'track')::int — errors
//     on a non-numeric string and fails the whole query.
//   - api/dbv1/jsonb_types.go's PlaylistContents.TrackIDs.Track is int64,
//     so the JSON Scan errors and propagates out of GetPlaylists.
//
// Either way, the playlist reads empty from the user's perspective.
//
// This hook runs after the Create/Update handler in the same DB
// transaction. It reads the playlist_contents we just wrote, swaps any
// string `track`/`track_id` values for their decoded integers via
// trashid.DecodeHashId (same Hashids salt/min_length used by ETL's
// junction-table path), and writes back only if something changed. Rows
// where every track is already an integer are left untouched, so the
// hook is a no-op for the common case.
//
// Errors are logged and swallowed — same "log and continue" contract the
// existing user_pubkey_hook follows. A failed normalization must not
// roll back the playlist row.
func newPlaylistContentsHook(logger *zap.Logger) em.PostHook {
	hookLogger := logger.Named("PlaylistContentsHook")

	return func(ctx context.Context, params *em.Params) error {
		return decodeHashidTracksInPlaylistContents(ctx, hookLogger, params)
	}
}

func decodeHashidTracksInPlaylistContents(ctx context.Context, logger *zap.Logger, params *em.Params) error {
	// Only run when the dispatched tx actually carried playlist_contents.
	// Updates that touch only the name/description don't rewrite the JSONB
	// column, so there's nothing for us to fix.
	if params.Metadata == nil {
		return nil
	}
	if _, ok := params.Metadata["playlist_contents"]; !ok {
		return nil
	}

	var contentsRaw []byte
	err := params.DBTX.QueryRow(ctx,
		`SELECT playlist_contents FROM playlists
		 WHERE playlist_id = $1 AND is_current = true`,
		params.EntityID,
	).Scan(&contentsRaw)
	if err != nil {
		// Row may have been removed earlier in the same tx (delete-after-update
		// in the same block) or the handler's UPDATE may have matched no rows.
		// Nothing we can normalize either way.
		logger.Debug("playlist_contents read missed; skipping normalization",
			zap.Int64("playlist_id", params.EntityID),
			zap.Error(err))
		return nil
	}
	if len(contentsRaw) == 0 {
		return nil
	}

	var contents struct {
		TrackIDs []map[string]any `json:"track_ids"`
	}
	if err := json.Unmarshal(contentsRaw, &contents); err != nil {
		// The upstream normalizer should always produce
		// {"track_ids":[...]}; if it didn't, this is an unfamiliar shape
		// and we shouldn't try to second-guess it.
		logger.Warn("playlist_contents JSONB not in {track_ids:[...]} shape; skipping",
			zap.Int64("playlist_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	changed := false
	for _, entry := range contents.TrackIDs {
		// Match the upstream junction-table decoder's key precedence
		// (pickPlaylistTrackID checks track_id first, then track).
		for _, key := range []string{"track_id", "track"} {
			raw, present := entry[key]
			if !present {
				continue
			}
			strVal, isString := raw.(string)
			if !isString || strVal == "" {
				continue
			}
			decoded, decodeErr := trashid.DecodeHashId(strVal)
			if decodeErr != nil {
				logger.Warn("failed to decode hashid in playlist_contents; leaving as-is",
					zap.Int64("playlist_id", params.EntityID),
					zap.String("key", key),
					zap.String("raw", strVal),
					zap.Error(decodeErr))
				continue
			}
			entry[key] = int64(decoded)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	rewritten, err := json.Marshal(map[string]any{"track_ids": contents.TrackIDs})
	if err != nil {
		logger.Warn("failed to remarshal normalized playlist_contents",
			zap.Int64("playlist_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	if _, err := params.DBTX.Exec(ctx,
		`UPDATE playlists SET playlist_contents = $1
		 WHERE playlist_id = $2 AND is_current = true`,
		rewritten, params.EntityID,
	); err != nil {
		logger.Warn("failed to write decoded playlist_contents",
			zap.Int64("playlist_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	logger.Info("decoded hashid track IDs in playlist_contents",
		zap.Int64("playlist_id", params.EntityID))
	return nil
}
