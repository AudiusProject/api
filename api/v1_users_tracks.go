package api

import (
	"fmt"
	"strings"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersTracksParams struct {
	Limit         int    `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	Sort          string `query:"sort" default:"date" validate:"oneof=date plays"`
	SortMethod    string `query:"sort_method" default:"" validate:"omitempty,oneof=title release_date plays reposts saves"`
	FilterTracks  string `query:"filter_tracks" default:"all" validate:"oneof=all public"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1UserTracks(c *fiber.Ctx) error {
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

	trackFilter := "(t.is_unlisted = false OR t.owner_id = @my_id)"
	switch params.FilterTracks {
	case "public":
		trackFilter = "t.is_unlisted = false"
	}

	// Gate condition filtering
	gateConditions := queryMulti(c, "gate_condition")
	gateFilter := buildGateConditionFilter(gateConditions)

	sql := `
	SELECT track_id
	FROM tracks t
	JOIN users u ON owner_id = u.user_id
	LEFT JOIN aggregate_plays ON track_id = play_item_id
	LEFT JOIN aggregate_track USING (track_id)
	WHERE t.owner_id = @user_id
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND (t.is_available = true OR t.owner_id = @my_id)
	  AND ` + trackFilter + `
	  AND t.stem_of is null
	  AND (t.access_authorities IS NULL
	    OR (COALESCE(@authed_wallet, '') <> ''
	        AND EXISTS (SELECT 1 FROM unnest(t.access_authorities) aa WHERE lower(aa) = lower(@authed_wallet))))` + gateFilter + `
	ORDER BY (CASE WHEN t.track_id = u.artist_pick_track_id THEN 0 ELSE 1 END), ` + orderClause + `
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

// buildGateConditionFilter builds SQL filter conditions based on gate_condition query parameters.
// Supports multiple conditions:
// - ungated: tracks with no gate
// - usdc_purchase: tracks with USDC purchase gate
// - follow: tracks with follow gate
// - tip: tracks with tip gate
// - nft: tracks with NFT gate
// - token: tracks with token gate
func buildGateConditionFilter(gateConditions []string) string {
	if len(gateConditions) == 0 {
		return ""
	}

	var conditions []string
	for _, condition := range gateConditions {
		switch condition {
		case "ungated":
			conditions = append(conditions, "(t.is_stream_gated = false)")
		case "usdc_purchase":
			conditions = append(conditions, "(t.is_stream_gated = true AND t.stream_conditions->>'usdc_purchase' IS NOT NULL)")
		case "follow":
			conditions = append(conditions, "(t.is_stream_gated = true AND t.stream_conditions->>'follow_user_id' IS NOT NULL)")
		case "tip":
			conditions = append(conditions, "(t.is_stream_gated = true AND t.stream_conditions->>'tip_user_id' IS NOT NULL)")
		case "nft":
			conditions = append(conditions, "(t.is_stream_gated = true AND t.stream_conditions->>'nft_collection' IS NOT NULL)")
		case "token":
			conditions = append(conditions, "(t.is_stream_gated = true AND t.stream_conditions->>'token_gate' IS NOT NULL)")
		}
	}

	if len(conditions) == 0 {
		return ""
	}

	// Join multiple conditions with OR and wrap in parentheses
	return "\n	  AND (" + strings.Join(conditions, " OR ") + ")"
}
