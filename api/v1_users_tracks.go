package api

import (
	"fmt"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersTracksParams struct {
	Limit            int    `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset           int    `query:"offset" default:"0" validate:"min=0"`
	Sort             string `query:"sort" default:"date" validate:"oneof=date plays"`
	SortMethod       string `query:"sort_method" default:"" validate:"omitempty,oneof=title release_date plays reposts saves"`
	FilterTracks     string `query:"filter_tracks" default:"all" validate:"omitempty,oneof=all public unlisted"`
	SortDirection    string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
	AiAttributedOnly bool   `query:"ai_attributed_only" default:"false"`
}

func (app *ApiServer) getUserTracks(c *fiber.Ctx, params GetUsersTracksParams) error {
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
	case "unlisted":
		trackFilter = "t.is_unlisted = true"
	}

	// ai_attributed case is tracks by any user which are attributed to
	// the target user handle/id, so swap the owner filter for that.
	ownerFilter := "t.owner_id = @user_id"
	if params.AiAttributedOnly {
		ownerFilter = "t.ai_attribution_user_id = @user_id"
	}

	sql := `
	SELECT track_id
	FROM tracks t
	JOIN users u ON owner_id = u.user_id
	LEFT JOIN aggregate_plays ON track_id = play_item_id
	LEFT JOIN aggregate_track USING (track_id)
	WHERE ` + ownerFilter + `
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND t.is_available = true
	  AND ` + trackFilter + `
	  AND t.stem_of is null
	ORDER BY ` + orderClause + `
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"user_id": userId,
		"my_id":   myId,
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

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  ids,
			MyID: myId,
		},
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}

func (app *ApiServer) v1UserTracks(c *fiber.Ctx) error {
	params := GetUsersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	return app.getUserTracks(c, params)
}

func (app *ApiServer) v1UserTracksAiAttributed(c *fiber.Ctx) error {
	params := GetUsersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	params.AiAttributedOnly = true
	return app.getUserTracks(c, params)
}
