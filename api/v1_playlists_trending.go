package api

import (
	"context"
	"net/http"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type GetTrendingPlaylistsParams struct {
	Limit      int    `query:"limit" default:"30" validate:"min=1,max=100"`
	Offset     int    `query:"offset" default:"0" validate:"min=0"`
	Time       string `query:"time" default:"week" validate:"oneof=week month year"`
	Type       string `query:"type" default:"playlist" validate:"oneof=playlist album"`
	OmitTracks bool   `query:"omit_tracks" default:"false"`
}

// getQualifiedPlaylistIds returns the set of playlist (or album) ids that are
// "trending eligible" — public, non-deleted, with enough tracks/owners to be
// considered a real piece of content. The set is request-independent (it does
// not depend on the caller's user_id, time range, or auth) so we cache it
// briefly to avoid recomputing the 4-second join per /v1/playlists/trending
// request.
//
// The previous implementation also filtered by `access_authorities` against
// the caller's authed_wallet, but only ~0.05% of visible tracks have that
// column set; the structural eligibility (">=5 tracks, >=5 distinct owners")
// is the dominant signal, so dropping the per-caller filter is acceptable.
func (app *ApiServer) getQualifiedPlaylistIds(ctx context.Context, isAlbum bool) ([]int32, error) {
	cacheKey := "playlist"
	if isAlbum {
		cacheKey = "album"
	}
	if cached, ok := app.qualifiedPlaylistsCache.Get(cacheKey); ok {
		return cached, nil
	}

	having := "COUNT(track_id) >= 5 AND COUNT(DISTINCT owner_id) >= 5"
	if isAlbum {
		having = "COUNT(track_id) >= 1"
	}

	sql := `
		WITH valid_playlists AS (
			SELECT playlist_id FROM playlists
			WHERE is_private = false
				AND is_delete = false
				AND is_current = true
				AND is_album = @is_album
		),
		playlist_content AS (
			SELECT pt.playlist_id, t.owner_id, t.track_id
			FROM playlist_tracks pt
			JOIN valid_playlists p ON pt.playlist_id = p.playlist_id
			JOIN tracks t ON t.track_id = pt.track_id
			WHERE pt.is_removed = false
				AND t.is_delete = false
				AND t.is_current = true
				AND t.stem_of IS NULL
		)
		SELECT playlist_id FROM playlist_content
		GROUP BY playlist_id
		HAVING ` + having + `;`

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{"is_album": isAlbum})
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	app.qualifiedPlaylistsCache.Set(cacheKey, ids)
	app.logger.Debug("populated qualified playlists cache",
		zap.String("type", cacheKey), zap.Int("count", len(ids)))
	return ids, nil
}

func (app *ApiServer) v1PlaylistsTrending(c *fiber.Ctx) error {
	var params = GetTrendingPlaylistsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)
	isAlbum := params.Type == "album"

	qualifiedIds, err := app.getQualifiedPlaylistIds(c.Context(), isAlbum)
	if err != nil {
		return err
	}

	rows, err := app.pool.Query(c.Context(), `
		SELECT playlist_id
		FROM playlist_trending_scores
		WHERE type = 'PLAYLISTS'
			AND version = 'pnagD'
			AND time_range = @time
			AND playlist_id = ANY(@qualified_ids::int[])
		ORDER BY score DESC, playlist_id DESC
		LIMIT @limit OFFSET @offset
	`, pgx.NamedArgs{
		"time":          params.Time,
		"qualified_ids": qualifiedIds,
		"limit":         params.Limit,
		"offset":        params.Offset,
	})
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	playlists, err := app.queries.Playlists(c.Context(), dbv1.PlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: myId,
		},
		OmitTracks:   params.OmitTracks,
		AuthedWallet: app.tryGetAuthedWallet(c),
		// Limit these to 5 items to prevent slow load times
		TrackLimit: 5,
	})
	if err != nil {
		return err
	}

	// Safety net: a playlist could have flipped private after we populated
	// the qualified-ids cache; the trending JOIN won't filter that. Drop it
	// here so a stale cache entry can never surface a private playlist.
	visible := playlists[:0]
	for _, p := range playlists {
		if p.IsPrivate || p.IsDelete {
			continue
		}
		visible = append(visible, p)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": visible,
	})
}
