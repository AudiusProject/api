package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type RewardCodeResponse struct {
	Code   string `json:"code"`
	Amount int64  `json:"amount"`
	IsUsed bool   `json:"-"` // Don't include in JSON response
}

type RewardAmountResponse struct {
	Amount int `json:"amount"`
}

// v1CoinsRedeem returns a static amount value
func (app *ApiServer) v1CoinsRedeem(c *fiber.Ctx) error {
	return c.JSON(RewardAmountResponse{
		Amount: 1,
	})
}

// v1CoinsRedeemCode retrieves a specific reward code for a mint
func (app *ApiServer) v1CoinsRedeemCode(c *fiber.Ctx) error {
	mint := c.Params("mint")
	code := c.Params("code")

	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{})
	}

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{})
	}

	sql := `
		SELECT code, amount, is_used
		FROM reward_codes
		WHERE mint = @mint AND code = @code
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
		"code": code,
	})
	if err != nil {
		return err
	}

	rewardCode, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[RewardCodeResponse])
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid",
			})
		}
		return err
	}

	if rewardCode.IsUsed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "used",
		})
	}

	return c.JSON(rewardCode)
}
