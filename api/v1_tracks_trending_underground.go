package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrendingUndergroundTracksParams struct {
	Limit  int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=500"`
	Time   string `query:"time" default:"week" validate:"oneof=week month allTime"`
	Genre  string `query:"genre" default:""`
}

func (app *ApiServer) v1TracksTrendingUnderground(c *fiber.Ctx) error {
	var params = GetTrendingUndergroundTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sql := `
	    WITH trending_tracks AS (
			SELECT track_trending_scores.track_id
			FROM track_trending_scores
			LEFT JOIN tracks
				ON tracks.track_id = track_trending_scores.track_id
				AND tracks.is_delete = false
				AND tracks.is_unlisted = false
				AND tracks.is_available = true
			WHERE type = 'TRACKS'
				AND version = 'pnagD'
				AND time_range = @time
				AND (@genre = '' OR track_trending_scores.genre = @genre)
			ORDER BY
				score DESC,
				track_id DESC
			LIMIT 100
		)

		SELECT track_trending_scores.track_id
		FROM track_trending_scores
		LEFT JOIN tracks
			ON tracks.track_id = track_trending_scores.track_id
			AND tracks.is_delete = false
			AND tracks.is_unlisted = false
			AND tracks.is_available = true
		LEFT JOIN aggregate_user
			ON aggregate_user.user_id = tracks.owner_id
		WHERE type = 'TRACKS'
			AND version = 'pnagD'
			AND time_range = @time
			AND (@genre = '' OR track_trending_scores.genre = @genre)
			AND aggregate_user.follower_count < 1500
			AND aggregate_user.following_count < 1500
			AND NOT EXISTS (
				SELECT 1
				FROM trending_tracks
				WHERE trending_tracks.track_id = track_trending_scores.track_id
			)
		ORDER BY
			track_trending_scores.score DESC,
			track_trending_scores.track_id DESC
		LIMIT @limit
		OFFSET @offset
		`
	args := pgx.NamedArgs{}
	args["limit"] = params.Limit
	args["offset"] = params.Offset
	args["time"] = params.Time
	args["genre"] = params.Genre

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
