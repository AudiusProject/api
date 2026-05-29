package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetChallengesInfoQueryParams struct {
	WeeklyPoolMinAmount *int `query:"weekly_pool_min_amount" validate:"omitempty,min=0"`
}

type ChallengeInfo struct {
	ChallengeID         string `json:"challenge_id" db:"id"`
	Type                string `json:"type" db:"type"`
	Amount              string `json:"amount" db:"amount"`
	Active              bool   `json:"active" db:"active"`
	StepCount           *int   `json:"step_count" db:"step_count"`
	StartingBlock       *int   `json:"starting_block" db:"starting_block"`
	WeeklyPool          *int   `json:"weekly_pool" db:"weekly_pool"`
	WeeklyPoolRemaining *int   `json:"weekly_pool_remaining" db:"weekly_pool_remaining"`
	CooldownDays        *int   `json:"cooldown_days" db:"cooldown_days"`
}

func getWeeklyPoolWindowStart() time.Time {
	now := time.Now().UTC()

	if now.Weekday() == time.Monday && now.Hour() < 16 {
		year, month, day := now.AddDate(0, 0, -7).Date()
		return time.Date(year, month, day, 16, 0, 0, 0, time.UTC)
	} else {
		// get monday of this week
		year, month, day := now.AddDate(0, 0, -int(now.Weekday())+1).Date()
		return time.Date(year, month, day, 16, 0, 0, 0, time.UTC)
	}
}

func (app *ApiServer) v1ChallengesInfo(c *fiber.Ctx) error {
	params := GetChallengesInfoQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	weeklyPoolWindowStart := getWeeklyPoolWindowStart()
	fmt.Println("weeklyPoolWindowStart", weeklyPoolWindowStart)

	sql := `
	  SELECT
	  	c.id,
		c.type,
		c.amount,
		c.active,
		c.step_count,
		c.starting_block,
		c.weekly_pool,
		c.cooldown_days,
		CASE
			WHEN c.weekly_pool IS NULL THEN NULL
			ELSE c.weekly_pool - COALESCE(
				-- Sum from the raw table, not the view, which would drop
				-- unresolved-recipient disbursements and overstate the remaining pool.
				(SELECT SUM(cd.amount) / 100000000
				 FROM sol_reward_disbursements cd
				 WHERE cd.challenge_id = c.id
				   AND cd.created_at > @weeklyPoolWindowStart),
				0
			)::int
		END as weekly_pool_remaining
	  FROM challenges c
	  WHERE c.id = @challengeId
	`

	challengeInfo := ChallengeInfo{}
	err := app.pool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"challengeId":           c.Params("challengeId"),
		"weeklyPoolWindowStart": weeklyPoolWindowStart,
	}).Scan(
		&challengeInfo.ChallengeID,
		&challengeInfo.Type,
		&challengeInfo.Amount,
		&challengeInfo.Active,
		&challengeInfo.StepCount,
		&challengeInfo.StartingBlock,
		&challengeInfo.WeeklyPool,
		&challengeInfo.CooldownDays,
		&challengeInfo.WeeklyPoolRemaining,
	)
	if err != nil {
		return err
	}

	// Quirk of this endpoint, send a 500 if the caller specified a weekly pool min amount and
	// it has been exceeded.
	if params.WeeklyPoolMinAmount != nil &&
		challengeInfo.WeeklyPoolRemaining != nil &&
		*challengeInfo.WeeklyPoolRemaining < *params.WeeklyPoolMinAmount {
		c.Status(500)
	}

	return c.JSON(fiber.Map{
		"data": challengeInfo,
	})
}
