package api

import (
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1ChallengesUndisbursed(c *fiber.Ctx) error {
	sql := `
	-- Get Undisbursed Challenges
	SELECT user_challenges.challenge_id,
		user_challenges.user_id,
		user_challenges.specifier,
		user_challenges.amount,
		user_challenges.completed_blocknumber,
		users.handle,
		users.wallet,
		user_challenges.created_at,
		user_challenges.completed_at,
		challenges.cooldown_days
	FROM user_challenges
	JOIN challenges ON challenges.id = user_challenges.challenge_id
	JOIN users ON users.user_id = user_challenges.user_id
	LEFT JOIN challenge_disbursements ON 
		challenge_disbursements.challenge_id = user_challenges.challenge_id
		AND challenge_disbursements.specifier = user_challenges.specifier
	WHERE challenge_disbursements.challenge_id IS NULL
		AND user_challenges.is_complete
		AND challenges.active
		AND users.is_current
		AND NOT users.is_deactivated
		AND (@user_id <= 0 OR user_challenges.user_id = @user_id)
		AND (@completed_blocknumber <= 0 OR user_challenges.completed_blocknumber > @completed_blocknumber)
		AND (@challenge_id = '' OR user_challenges.challenge_id = @challenge_id)
	ORDER BY
		user_challenges.user_id ASC,
		user_challenges.challenge_id ASC,
		user_challenges.completed_blocknumber ASC
	LIMIT COALESCE(@limit, 100)
	OFFSET @offset
	;
	`

	type GetUndisbursedChallengesQueryParams struct {
		ChallengeID          string `query:"challenge_id"`
		CompletedBlocknumber int    `query:"completed_blocknumber"`
		Limit                *int   `query:"limit"`
		Offset               int    `query:"offset"`
	}
	params := GetUndisbursedChallengesQueryParams{}
	err := c.QueryParser(&params)
	if err != nil {
		return err
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id":               app.getMyId(c),
		"completed_blocknumber": params.CompletedBlocknumber,
		"challenge_id":          params.ChallengeID,
		"limit":                 params.Limit,
		"offset":                params.Offset,
	})
	if err != nil {
		return err
	}

	type GetUndisbursedChallengesRow struct {
		ChallengeID          string         `json:"challenge_id"`
		UserID               trashid.HashId `json:"user_id"`
		Specifier            string         `json:"specifier"`
		Amount               string         `json:"amount"`
		CompletedBlocknumber *int           `json:"completed_blocknumber"`
		Handle               string         `json:"handle"`
		Wallet               string         `json:"wallet"`
		CreatedAt            time.Time      `json:"created_at"`
		CompletedAt          *time.Time     `json:"completed_at"`
		CooldownDays         *int           `json:"cooldown_days"`
	}
	res, err := pgx.CollectRows(rows, pgx.RowToStructByName[GetUndisbursedChallengesRow])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": res,
	})
}
