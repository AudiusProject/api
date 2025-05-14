package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *ApiServer) v1UsersChallenges(c *fiber.Ctx) error {
	sql := `
	-- Start with the user ID and verification status
	WITH user_row AS (
		SELECT 
			user_id, 
			is_verified
		FROM users 
		WHERE users.is_current 
			AND users.user_id = @userId
	),

	-- Pre-filter to their user challenges
	user_challenges_filtered AS (
		SELECT * FROM user_challenges JOIN user_row USING (user_id)
	),

	-- Pre-filter to their disbursements
	challenge_disbursements_filtered AS (
		SELECT * FROM challenge_disbursements JOIN user_row USING (user_id)
	),

	-- Start with the list of all active challenges, and then
	-- apply the user's user challenges and disbursements.
	-- Filter out incomplete trending challenges,
	-- verified-only challenges if not verified
	-- and non-verified only challenges if verified.
	all_user_challenges AS (
		SELECT challenges.id AS challenge_id,
			user_row.user_id,
			COALESCE(user_challenges_filtered.specifier, '') AS specifier,
			COALESCE(user_challenges_filtered.is_complete, false) AS is_complete,
			challenges.active AS is_active,
			(challenge_disbursements_filtered.slot IS NOT NULL) AS is_disbursed,
			COALESCE(user_challenges_filtered.current_step_count, 0) AS current_step_count,
			challenges.step_count AS max_steps,
			challenges.type AS challenge_type,
			COALESCE(challenges.amount::BIGINT, 0) AS amount,
			COALESCE(user_challenges_filtered.amount, 0) AS user_amount,
			COALESCE(challenge_disbursements_filtered.amount, '0')::BIGINT / 100000000 AS disbursed_amount,
			COALESCE(challenges.cooldown_days, 0) AS cooldown_days,
			user_challenges_filtered.created_at
		FROM challenges
		LEFT JOIN user_challenges_filtered ON challenges.id = user_challenges_filtered.challenge_id
		CROSS JOIN user_row
		LEFT JOIN challenge_disbursements_filtered
			ON user_challenges_filtered.challenge_id = challenge_disbursements_filtered.challenge_id 
			AND user_challenges_filtered.specifier = challenge_disbursements_filtered.specifier
		WHERE challenges.active
			AND (challenges.type != 'trending' OR user_challenges_filtered.is_complete)
			AND NOT (challenges.id IN ('rv', 's') AND NOT user_row.is_verified)
			AND NOT (challenges.id IN ('r') AND user_row.is_verified)
	),

	-- Get the most recent listen streak summary
	current_listen_streak AS (
		SELECT
			listen_streak,
			last_listen_date
		FROM challenge_listen_streak
		JOIN user_row USING (user_id)
		WHERE NOW() - last_listen_date < INTERVAL '16' HOUR
	)

	-- Non-aggregate challenges just use the values as-is
	SELECT challenge_id,
		user_id,
		specifier,
		is_complete,
		is_active,
		is_disbursed,
		current_step_count,
		max_steps,
		challenge_type,
		amount,
		disbursed_amount,
		cooldown_days
	FROM all_user_challenges
	WHERE challenge_type != 'aggregate'

	-- Aggregate challenges besides endless listen streak and oneshot need
	-- to be rolled up. Sum the amounts, check for completion, etc.
	UNION ALL (
		SELECT challenge_id,
			user_id,
			'' AS specifier,
			SUM(user_amount) >= max_steps AS is_complete,
			is_active,
			false AS is_disbursed,
			SUM(user_amount) AS current_step_count,
			max_steps,
			challenge_type,
			amount,
			SUM(disbursed_amount) AS disbursed_amount,
			cooldown_days
		FROM all_user_challenges
		WHERE challenge_type = 'aggregate'
			AND challenge_id NOT IN ('e', 'o')
		GROUP BY challenge_id,
			user_id,
			max_steps,
			is_active,
			challenge_type,
			amount,
			cooldown_days
	)

	-- Endless listen streak needs some custom logic to get step count, amount
	UNION ALL (
		SELECT DISTINCT ON (challenge_id)
			challenge_id,
			user_id,
			'' AS specifier,
			COALESCE(current_listen_streak.listen_streak, 0) > 0 AND all_user_challenges.is_complete AS is_complete,
			is_active,
			false AS is_disbursed,
			COALESCE(
					current_listen_streak.listen_streak,
					0
				) AS current_step_count,
			max_steps,
			challenge_type,
			GREATEST(all_user_challenges.user_amount, all_user_challenges.amount) AS amount,
			(SELECT SUM(disbursed_amount) 
					FROM all_user_challenges 
					WHERE challenge_id = 'e'
				) AS disbursed_amount,
			cooldown_days
		FROM all_user_challenges
		LEFT JOIN current_listen_streak ON 1=1
		WHERE all_user_challenges.challenge_id = 'e'
		ORDER BY all_user_challenges.challenge_id, created_at DESC
	)

	-- Oneshot is unique per user, always complete, amount 1 if user challenge exists.
	UNION ALL (
		SELECT challenge_id,
			user_id,
			'' AS specifier,
			SUM(user_amount) > 0 AS is_complete,
			is_active,
			false AS is_disbursed,
			SUM(user_amount) AS current_step_count,
			CASE WHEN SUM(user_amount) = 0 
					THEN max_steps 
					ELSE SUM(user_amount) 
				END AS max_steps,
			challenge_type,
			1 AS amount,
			SUM(disbursed_amount) AS disbursed_amount,
			cooldown_days
		FROM all_user_challenges
		WHERE challenge_id = 'o'
		GROUP BY challenge_id,
			user_id,
			max_steps,
			is_active,
			challenge_type,
			amount,
			cooldown_days
	)
	;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
	})
	if err != nil {
		return err
	}

	type GetUserChallengesRow struct {
		ChallengeID      string         `json:"challenge_id"`
		UserID           trashid.HashId `json:"user_id"`
		Specifier        string         `json:"specifier"`
		IsComplete       bool           `json:"is_complete"`
		IsActive         bool           `json:"is_active"`
		IsDisbursed      bool           `json:"is_disbursed"`
		CurrentStepCount int32          `json:"current_step_count"`
		MaxSteps         pgtype.Int4    `json:"max_steps"`
		ChallengeType    string         `json:"challenge_type"`
		Amount           string         `json:"amount"`
		DisbursedAmount  uint64         `json:"disbursed_amount"`
		CooldownDays     int32          `json:"cooldown_days"`
	}

	res, err := pgx.CollectRows(rows, pgx.RowToStructByName[GetUserChallengesRow])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": res,
	})
}
