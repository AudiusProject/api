package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"api.audius.co/solana/spl/programs/meteora_dbc"
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
	User *dbv1.FullUser `json:"user"`
}

type VolumeLeaderUserMin struct {
	VolumeLeaderRow
	User *dbv1.MinUser `json:"user"`
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

	sql := `
	-- Get pool vaults for both DBC and DAMM V2 to match against
	WITH pool_vaults AS (
	SELECT quote_vault AS vault_account, 'dbc' AS src FROM sol_meteora_dbc_pools
	UNION
	SELECT token_b_vault AS vault_account, 'damm_v2' AS src FROM sol_meteora_damm_v2_pools
	),
	leaders as (SELECT
		user_change.owner,
		user_change.account,
		SUM(ABS(user_change.change)) / 100000000 AS volume  -- dividing by AUDIO decimals
	FROM sol_token_account_balance_changes user_change
	JOIN sol_token_account_balance_changes vault_change
		ON vault_change.signature = user_change.signature
		AND vault_change.change = 0 - user_change.change
		AND vault_change.mint = user_change.mint
	JOIN pool_vaults pv
    	ON pv.vault_account = vault_change.account
	WHERE
		user_change.owner != '` + meteora_dbc.POOL_AUTHORITY_ADDRESS + `'
		AND user_change.owner != '` + meteora_damm_v2.POOL_AUTHORITY_ADDRESS + `'
		AND vault_change.created_at >= @fromDate
		AND vault_change.created_at < @toDate
		AND user_change.created_at >= @fromDate
		AND user_change.created_at < @toDate
	GROUP BY user_change.owner, user_change.account
	ORDER BY volume DESC
	)
	select
	    COALESCE(u.user_id, aw.user_id) as user_id,
		-- Use account address if it's a claimable tokens ATA
    	COALESCE(sca.account, l.owner) as address,
    	l.volume
    FROM leaders l
    LEFT JOIN associated_wallets aw ON aw.wallet = l.owner
    LEFT JOIN sol_claimable_accounts  sca ON sca.account = l.account
    LEFT JOIN users u ON u.wallet = sca.ethereum_address
    WHERE l.volume > 0
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

	users, err := app.queries.FullUsersKeyed(c.Context(), dbv1.GetUsersParams{
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
					minUser := dbv1.ToMinUser(user)
					results[i].User = &minUser
				}
			}
		}
		return c.JSON(fiber.Map{
			"data": results,
		})
	}
}
