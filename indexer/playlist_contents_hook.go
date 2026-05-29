package indexer

import (
	"context"
	"encoding/json"

	"api.audius.co/trashid"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"go.uber.org/zap"
)

// newPlaylistContentsHook returns a pkg/etl post-hook that normalizes the
// playlists.playlist_contents JSONB column so it matches the canonical
// shape API readers expect, regardless of which field names and value
// types the SDK uploaded.
//
// Why this exists: the TypeScript SDK uploads playlist contents in its
// own camelCase shape and snakecase-keys converts that to snake_case
// before signing. The on-chain metadata therefore carries entries like:
//
//	{"track_id": "1aV5byE", "timestamp": 1700000000, "metadata_timestamp": ...}
//
// (see audius-protocol packages/sdk/.../playlists/types.ts — the schema
// is `{ trackId: HashId, timestamp, metadataTimestamp? }`). ETL's
// `normalizePlaylistContentsJSON` writes those entries to the JSONB
// verbatim. The downstream API readers in this repo, however, expect the
// canonical shape that the legacy Python apps indexer always wrote:
//
//	{"track": <int>, "time": <int>, "metadata_time": <int>}
//
//   - api/v1_playlist_tracks.go casts (e.value->>'track')::int — returns
//     NULL when the column has `track_id` instead of `track`, which then
//     fails the JOIN to tracks and the endpoint returns no rows.
//   - api/dbv1/jsonb_types.go's PlaylistContents.TrackIDs.Track is
//     int64, so JSON Scan returns 0 for any missing `track` field. Zero
//     never matches a real track ID, so the playlist's tracks slice
//     comes back empty.
//
// Either path produces "playlist is empty after a refresh" — which is
// exactly the prod report this hook is fixing.
//
// What it does: after a successful Playlist Create or Update, re-read
// the JSONB inside the same transaction and, per entry:
//
//   - Resolve a `track` integer (from existing `track`/`track_id`,
//     decoding via trashid.DecodeHashId for hashid strings).
//   - Copy `timestamp` to `time` if `time` is missing.
//   - Copy `metadata_timestamp` (falling back to `timestamp`) to
//     `metadata_time` if missing.
//   - Drop the SDK-style `track_id`/`timestamp`/`metadata_timestamp`
//     keys after copying their values, matching the Python indexer's
//     historical canonical output.
//
// Rows that already have the canonical shape (every entry has integer
// `track` + `time` + `metadata_time`, no SDK keys) are left byte-
// identical — no spurious UPDATE.
//
// Errors are logged and swallowed per the upstream PostHook contract;
// failure to normalize must not roll back the playlist row.
//
// This belongs upstream in normalizePlaylistContentsJSON itself
// (open follow-up to go-openaudio). Patching here lands the fix on
// this repo's CI/CD now without waiting for an ETL bump.
func newPlaylistContentsHook(logger *zap.Logger) em.PostHook {
	hookLogger := logger.Named("PlaylistContentsHook")

	return func(ctx context.Context, params *em.Params) error {
		return normalizePlaylistContentsRow(ctx, hookLogger, params)
	}
}

func normalizePlaylistContentsRow(ctx context.Context, logger *zap.Logger, params *em.Params) error {
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
		if normalizeEntry(entry, logger, params.EntityID) {
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
		logger.Warn("failed to write normalized playlist_contents",
			zap.Int64("playlist_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	logger.Info("normalized playlist_contents to canonical shape",
		zap.Int64("playlist_id", params.EntityID))
	return nil
}

// normalizeEntry mutates a single playlist_contents entry in place,
// returning true if it made any change. Canonical entries (integer
// `track`, numeric `time` + `metadata_time`, no SDK keys) pass through
// untouched.
//
// Field-rename policy matches what apps' Python process_playlist_contents
// emits: `track` is the integer ID, `time` is the block-indexed time
// (we use whatever the SDK gave as `timestamp`/`metadata_timestamp` —
// the legacy block_integer_time isn't available in a post-hook), and
// `metadata_time` is the SDK-supplied metadata timestamp.
func normalizeEntry(entry map[string]any, logger *zap.Logger, playlistID int64) (changed bool) {
	// Track ID: target is integer `track`.
	if trackVal, ok := resolveTrackID(entry, logger, playlistID); ok {
		if !isInt64Value(entry["track"], trackVal) {
			entry["track"] = trackVal
			changed = true
		}
	}
	if _, has := entry["track_id"]; has {
		delete(entry, "track_id")
		changed = true
	}

	// time and metadata_time: copy from SDK-style fields if the canonical
	// fields are absent. Prefer `timestamp` for `time`; prefer
	// `metadata_timestamp` (with `timestamp` as fallback) for
	// `metadata_time`. This mirrors the apps Python fallback chain.
	if _, has := entry["time"]; !has {
		for _, src := range []string{"timestamp", "metadata_timestamp"} {
			if v, ok := entry[src].(float64); ok {
				entry["time"] = v
				changed = true
				break
			}
		}
	}
	if _, has := entry["metadata_time"]; !has {
		for _, src := range []string{"metadata_timestamp", "timestamp"} {
			if v, ok := entry[src].(float64); ok {
				entry["metadata_time"] = v
				changed = true
				break
			}
		}
	}

	// Drop SDK-style keys whose values we've already copied to canonical.
	for _, sdkKey := range []string{"timestamp", "metadata_timestamp"} {
		if _, has := entry[sdkKey]; has {
			delete(entry, sdkKey)
			changed = true
		}
	}

	return changed
}

// resolveTrackID returns the canonical integer track ID for an entry,
// looking first at `track` and falling back to `track_id`. String values
// are decoded via trashid.DecodeHashId (same Hashids salt + min length
// as the upstream junction-table decoder).
func resolveTrackID(entry map[string]any, logger *zap.Logger, playlistID int64) (int64, bool) {
	for _, key := range []string{"track", "track_id"} {
		raw, ok := entry[key]
		if !ok {
			continue
		}
		if v, ok := asInt64(raw); ok {
			return v, true
		}
		strVal, isString := raw.(string)
		if !isString || strVal == "" {
			continue
		}
		decoded, err := trashid.DecodeHashId(strVal)
		if err != nil {
			logger.Warn("failed to decode track ID in playlist_contents; leaving entry alone",
				zap.Int64("playlist_id", playlistID),
				zap.String("key", key),
				zap.String("raw", strVal),
				zap.Error(err))
			continue
		}
		return int64(decoded), true
	}
	return 0, false
}

func asInt64(raw any) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

func isInt64Value(raw any, target int64) bool {
	v, ok := asInt64(raw)
	return ok && v == target
}
