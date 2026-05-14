package api

import (
	"strings"
	"time"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ChallengeDisbursement struct {
	ChallengeID string         `json:"challenge_id"`
	UserID      trashid.HashId `json:"user_id"`
	Specifier   string         `json:"specifier"`
	Signature   string         `json:"signature"`
	Slot        int32          `json:"slot"`
	Amount      string         `json:"amount"`
	CreatedAt   time.Time      `json:"created_at"`
}

type GetChallengeDisbursementsQueryParams struct {
	ChallengeID     string         `query:"challenge_id"`
	SpecifierQuery  string         `query:"specifier_query" validate:"omitempty,min=1,max=255"`
	ChallengeUserID trashid.HashId `query:"challenge_user_id"`
	SortMethod      string         `query:"sort_method" default:"created_at" validate:"omitempty,oneof=created_at slot amount specifier user_id"`
	SortDirection   string         `query:"sort_direction" default:"desc" validate:"omitempty,oneof=asc desc"`
	Limit           int            `query:"limit" default:"10" validate:"min=1,max=500"`
	Offset          int            `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1ChallengesDisbursements(c *fiber.Ctx) error {
	params := GetChallengeDisbursementsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sortDir := "DESC"
	if params.SortDirection == "asc" {
		sortDir = "ASC"
	}

	sortMethod := "cd.created_at"
	switch params.SortMethod {
	case "created_at":
		sortMethod = "cd.created_at"
	case "slot":
		sortMethod = "cd.slot"
	case "amount":
		sortMethod = "cd.amount"
	case "specifier":
		sortMethod = "cd.specifier"
	case "user_id":
		sortMethod = "cd.user_id"
	}

	whereClause := ""
	filters := []string{}
	if params.ChallengeID != "" {
		filters = append(filters, "cd.challenge_id = @challengeId")
	}
	if params.SpecifierQuery != "" {
		filters = append(filters, "cd.specifier ILIKE '%' || @specifierQuery || '%'")
	}
	if params.ChallengeUserID != 0 {
		filters = append(filters, "cd.user_id = @challengeUserId")
	}

	if len(filters) > 0 {
		whereClause = "WHERE " + strings.Join(filters, " AND ")
	}

	sql := `
	SELECT
		cd.challenge_id,
		cd.user_id,
		cd.specifier,
		cd.amount,
		cd.created_at,
		cd.signature,
		cd.slot
	FROM v_challenge_disbursements cd
	` + whereClause + `
	ORDER BY ` + sortMethod + ` ` + sortDir + `, cd.user_id ASC
	LIMIT @limit
	OFFSET @offset;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"challengeId":     params.ChallengeID,
		"specifierQuery":  params.SpecifierQuery,
		"challengeUserId": params.ChallengeUserID,
		"limit":           params.Limit,
		"offset":          params.Offset,
	})
	if err != nil {
		return err
	}

	res, err := pgx.CollectRows(rows, pgx.RowToStructByName[ChallengeDisbursement])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": res,
	})
}
