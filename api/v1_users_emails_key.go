package api

import (
	"time"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type EmailAccessResponse struct {
	Id               int32     `json:"id" db:"id"`
	EmailOwnerUserId int32     `json:"email_owner_user_id" db:"email_owner_user_id"`
	ReceivingUserId  int32     `json:"receiving_user_id" db:"receiving_user_id"`
	GrantorUserId    int32     `json:"grantor_user_id" db:"grantor_user_id"`
	EncryptedKey     string    `json:"encrypted_key" db:"encrypted_key"`
	IsInitial        bool      `json:"is_initial" db:"is_initial"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

func (app *ApiServer) v1UsersEmailsKey(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	hashedGrantorUserId := c.Params("grantorUserId")

	grantorUserId, err := trashid.DecodeHashId(hashedGrantorUserId)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid grantor user ID")
	}

	sql := `SELECT * FROM email_access
	WHERE receiving_user_id=@userId
	AND grantor_user_id=@grantorUserId
	LIMIT 1`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":        userId,
		"grantorUserId": grantorUserId,
	})
	if err != nil {
		return err
	}

	emailAccess, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[EmailAccessResponse])
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.JSON(fiber.Map{
				"data": nil,
			})
		}
		return err
	}

	return c.JSON(fiber.Map{
		"data": emailAccess,
	})
}
