package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1UserTracksCount returns the count of tracks for a user with optional gate condition filtering.
// This is a lightweight endpoint that returns only the count without fetching full track data.
// Shares the same query parameters as v1UserTracks for consistent filtering.
//
// Query parameters:
//   - gate_condition: Optional, can be repeated. Values: ungated, usdc_purchase, follow, tip, nft, token
//   - filter_tracks: Optional. Values: all (default), public
//
// Example: GET /v1/full/users/:userId/tracks/count?gate_condition=token&gate_condition=usdc_purchase
func (app *ApiServer) v1UserTracksCount(c *fiber.Ctx) error {
	params := GetUsersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)
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
	SELECT COUNT(*)
	FROM tracks t
	JOIN users u ON owner_id = u.user_id
	WHERE t.owner_id = @user_id
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND t.is_available = true
	  AND ` + trackFilter + `
	  AND t.stem_of is null
	  AND (t.access_authorities IS NULL
	    OR (COALESCE(@authed_wallet, '') <> ''
	        AND EXISTS (SELECT 1 FROM unnest(t.access_authorities) aa WHERE lower(aa) = lower(@authed_wallet))))` + gateFilter

	args := pgx.NamedArgs{
		"user_id":       userId,
		"my_id":         myId,
		"authed_wallet": app.tryGetAuthedWallet(c),
	}

	var count int
	err := app.pool.QueryRow(c.Context(), sql, args).Scan(&count)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
