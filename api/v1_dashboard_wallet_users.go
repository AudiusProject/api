package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type DashboardWalletUsersParams struct {
	// TODO: wallets should be an array of strings rather than a comma separated string
	// but for backwards compat with discovery node, is as so
	Wallets string `query:"wallets" validate:"required"`
}

type DashboardWalletUser struct {
	Wallet string        `json:"wallet"`
	User   dbv1.FullUser `json:"user"`
}

type MinDashboardWalletUser struct {
	Wallet string       `json:"wallet"`
	User   dbv1.MinUser `json:"user"`
}

func (app *ApiServer) v1DashboardWalletUsers(c *fiber.Ctx) error {
	params := DashboardWalletUsersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	if params.Wallets == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no wallets provided")
	}

	walletStrs := strings.Split(params.Wallets, ",")
	lowerWallets := make([]string, len(walletStrs))
	for i, wallet := range walletStrs {
		lowerWallets[i] = strings.ToLower(strings.TrimSpace(wallet))
	}

	myId := app.getMyId(c)

	sql := `
		SELECT dwu.wallet, dwu.user_id
		FROM dashboard_wallet_users dwu
		WHERE LOWER(dwu.wallet) = ANY($1)
		AND dwu.is_delete = false
	`

	rows, err := app.pool.Query(c.Context(), sql, lowerWallets)
	if err != nil {
		return err
	}

	type walletUserRow struct {
		Wallet string `db:"wallet"`
		UserID int32  `db:"user_id"`
	}

	walletUserRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[walletUserRow])
	if err != nil {
		return err
	}

	if len(walletUserRows) == 0 {
		return c.JSON(fiber.Map{"data": []DashboardWalletUser{}})
	}
	userIds := make([]int32, len(walletUserRows))
	for i, row := range walletUserRows {
		userIds[i] = row.UserID
	}
	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{MyID: myId, Ids: userIds})
	if err != nil {
		return err
	}
	userMap := make(map[int32]dbv1.FullUser, len(users))
	for _, u := range users {
		userMap[u.UserID] = u
	}

	if app.getIsFull(c) {
		result := make([]DashboardWalletUser, 0, len(walletUserRows))
		for _, row := range walletUserRows {
			if user, ok := userMap[row.UserID]; ok {
				result = append(result, DashboardWalletUser{Wallet: row.Wallet, User: user})
			}
		}
		return c.JSON(fiber.Map{
			"data": result,
		})
	} else {
		result := make([]MinDashboardWalletUser, 0, len(walletUserRows))
		for _, row := range walletUserRows {
			if user, ok := userMap[row.UserID]; ok {
				result = append(result, MinDashboardWalletUser{Wallet: row.Wallet, User: dbv1.ToMinUser(user)})
			}
		}
		return c.JSON(fiber.Map{
			"data": result,
		})
	}
}
