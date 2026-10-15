package api

import (
	"fmt"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UserTracksAiAttributed(c *fiber.Ctx) error {
	params := GetUsersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sortDir := "DESC"
	if params.SortDirection == "asc" {
		sortDir = "ASC"
	}

	// Default sort is by legacy `sort:` param value of 'date'
	orderClause := fmt.Sprintf("coalesce(t.release_date, t.created_at) %s, t.track_id", sortDir)
	if params.SortMethod != "" {
		switch params.SortMethod {
		case "title":
			orderClause = fmt.Sprintf("t.title %s, t.track_id", sortDir)
		case "release_date":
			orderClause = fmt.Sprintf("coalesce(t.release_date, t.created_at) %s, t.track_id", sortDir)
		case "plays":
			orderClause = fmt.Sprintf("aggregate_plays.count %s, t.track_id", sortDir)
		case "reposts":
			orderClause = fmt.Sprintf("repost_count %s, t.track_id", sortDir)
		case "saves":
			orderClause = fmt.Sprintf("save_count %s, t.track_id", sortDir)
		}
	} else {
		switch params.Sort {
		case "plays":
			orderClause = fmt.Sprintf("aggregate_plays.count %s, t.track_id", sortDir)
		}
	}
	userId := app.getUserId(c)

	trackFilter := "t.is_unlisted = false OR t.owner_id = @my_id"
	switch params.FilterTracks {
	case "public":
		trackFilter = "t.is_unlisted = false"
	}

	sql := `
	SELECT track_id
	FROM tracks t
	JOIN users u ON owner_id = u.user_id
	LEFT JOIN aggregate_plays ON track_id = play_item_id
	LEFT JOIN aggregate_track USING (track_id)
	WHERE t.ai_attribution_user_id = @user_id
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND t.is_available = true
	  AND ` + trackFilter + `
	  AND t.stem_of is null
	  AND (t.access_authorities IS NULL
	    OR (COALESCE(@authed_wallet, '') <> ''
	        AND EXISTS (SELECT 1 FROM unnest(t.access_authorities) aa WHERE lower(aa) = lower(@authed_wallet))))
	ORDER BY ` + orderClause + `
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"user_id":       userId,
		"my_id":         myId,
		"authed_wallet": app.tryGetAuthedWallet(c),
	}
	args["limit"] = params.Limit
	args["offset"] = params.Offset

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          ids,
			MyID:         myId,
			AuthedWallet: app.tryGetAuthedWallet(c),
		},
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}
