package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type VolumeLeaderRow struct {
	Address string  `json:"address" db:"address"`
	Volume  float64 `json:"volume" db:"volume"`
	UserID  *int32  `json:"-" db:"user_id"`
}

type VolumeLeaderUserFull struct {
	VolumeLeaderRow
	User *dbv1.User `json:"user"`
}

type VolumeLeaderUserMin struct {
	VolumeLeaderRow
	User *dbv1.User `json:"user"`
}

type GetCoinsVolumeLeadersQueryParams struct {
	Limit int `query:"limit" default:"20" validate:"min=1,max=100"`
	// Max offset of 500 to prevent expensive queries
	Offset   int    `query:"offset" default:"0" validate:"min=0,max=500"`
	FromDate string `query:"from" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	ToDate   string `query:"to" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
}

func (app *ApiServer) v1CoinsVolumeLeaders(c *fiber.Ctx) error {
	queryParams := GetCoinsVolumeLeadersQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	now := time.Now().UTC()

	// Default to midnight of the current day (UTC)
	fromDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if queryParams.FromDate != "" {
		parsed, err := time.Parse(time.RFC3339, queryParams.FromDate)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid from date")
		}
		fromDate = parsed
	}

	toDate := fromDate.AddDate(0, 0, 1)
	if queryParams.ToDate != "" {
		parsed, err := time.Parse(time.RFC3339, queryParams.ToDate)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid to date")
		}
		toDate = parsed
	}

	if toDate.Before(fromDate) {
		return fiber.NewError(fiber.StatusBadRequest, "To date must be after from date")
	}

	// Semi-arbitrary, but meant to prevent huge queries across long time spans
	if toDate.Sub(fromDate) > 7*24*time.Hour {
		return fiber.NewError(fiber.StatusBadRequest, "Time range must be <= 7 days")
	}

	if fromDate.Before(now.Add(-11 * 24 * time.Hour)) {
		return fiber.NewError(fiber.StatusBadRequest, "Time range too old")
	}

	sql := `
	WITH pool_vaults AS (
		SELECT quote_vault AS vault_account, 'dbc' AS src FROM sol_meteora_dbc_pools
		UNION
		SELECT token_b_vault AS vault_account, 'damm_v2' AS src FROM sol_meteora_damm_v2_pools
	),
	excluded_addresses AS (
		SELECT address FROM volume_leader_exclusions
	),
    vault_change AS (
        SELECT *
        FROM sol_token_account_balance_changes stabc
        JOIN pool_vaults pv
    	ON pv.vault_account = stabc.account
        WHERE
			-- Leverage mint/block_timestamp index (we only care about AUDIO vaults)
            stabc.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
            AND stabc.block_timestamp >= @fromDate
            AND stabc.block_timestamp < @toDate
    ),
	leaders as (SELECT
		COALESCE(scat_match.account, vault_change.fee_payer) as address,
		SUM(ABS(vault_change.change)) / 100000000 AS volume  -- dividing by AUDIO decimals
	FROM vault_change
    LEFT JOIN sol_token_account_balance_changes user_change
		ON vault_change.signature = user_change.signature
		AND vault_change.change = 0 - user_change.change
		AND vault_change.mint = user_change.mint
	-- If the tx involved a claimable tokens transfer, it will get credited to
	-- the AUDIO user bank address for the user that sent the tx.
	LEFT JOIN LATERAL (
		SELECT DISTINCT ON (sca.account)
				sca.account
		FROM sol_claimable_account_transfers scat
		JOIN sol_claimable_accounts sca
			ON sca.ethereum_address = scat.sender_eth_address
			-- Return AUDIO claimable token account for sender with each matching tx.
			AND sca.mint = '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM'
		WHERE scat.signature = vault_change.signature
		ORDER BY sca.account, scat.instruction_index
    ) scat_match ON TRUE
	GROUP BY address
	ORDER BY volume DESC
	),
	leaders_with_users AS (
		SELECT
			COALESCE(u.user_id, aw.user_id) as user_id,
			l.address,
			l.volume,
			ROW_NUMBER() OVER (
				PARTITION BY COALESCE(u.user_id, aw.user_id)
				ORDER BY l.volume DESC
			) as wallet_rank
		FROM leaders l
		LEFT JOIN associated_wallets aw ON aw.wallet = l.address
		LEFT JOIN sol_claimable_accounts sca ON sca.account = l.address
		LEFT JOIN users u ON u.wallet = sca.ethereum_address
		WHERE
			l.address IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM excluded_addresses WHERE excluded_addresses.address = l.address)
			AND l.volume > 0
	)
	SELECT
		user_id,
		address,
		volume
	FROM leaders_with_users
	WHERE wallet_rank = 1
	ORDER BY volume DESC
	LIMIT @limit
	OFFSET @offset;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"fromDate": fromDate,
		"toDate":   toDate,
		"limit":    queryParams.Limit,
		"offset":   queryParams.Offset,
	})
	if err != nil {
		return err
	}

	leaders, err := pgx.CollectRows(rows, pgx.RowToStructByName[VolumeLeaderRow])
	if err != nil {
		return err
	}

	userIds := make([]int32, 0)
	for _, leader := range leaders {
		if leader.UserID != nil {
			userIds = append(userIds, *leader.UserID)
		}
	}

	users, err := app.queries.UsersKeyed(c.Context(), dbv1.GetUsersParams{
		Ids: userIds,
	})
	if err != nil {
		return err
	}

	if app.getIsFull(c) {
		results := make([]VolumeLeaderUserFull, len(leaders))
		for i, leader := range leaders {
			results[i] = VolumeLeaderUserFull{
				VolumeLeaderRow: leader,
				User:            nil,
			}
			if leader.UserID != nil {
				if user, ok := users[*leader.UserID]; ok {
					results[i].User = &user
				}
			}
		}
		return c.JSON(fiber.Map{
			"data": results,
		})
	} else {
		results := make([]VolumeLeaderUserMin, len(leaders))
		for i, leader := range leaders {
			results[i] = VolumeLeaderUserMin{
				VolumeLeaderRow: leader,
			}
			if leader.UserID != nil {
				if user, ok := users[*leader.UserID]; ok {
					minUser := user
					results[i].User = &minUser
				}
			}
		}
		return c.JSON(fiber.Map{
			"data": results,
		})
	}
}
