package api

import (
	"strconv"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1GranteeUsers(c *fiber.Ctx) error {
	address := c.Params("address")

	isApproved, err := getOptionalBool(c, "is_approved")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_approved")
	}

	isRevoked, err := strconv.ParseBool(c.Query("is_revoked", "false"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_revoked")
	}

	params := GetUsersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sql := `
	SELECT g.user_id
	FROM grants g
	WHERE g.grantee_address = @grantee_address
	  AND g.is_current = true
	  AND g.is_revoked = @is_revoked
	  AND (@is_approved::boolean IS NULL OR g.is_approved = @is_approved)
	ORDER BY g.created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"grantee_address": address,
		"is_revoked":      isRevoked,
		"is_approved":     isApproved,
		"limit":           params.Limit,
		"offset":          params.Offset,
	})
	if err != nil {
		return err
	}

	userIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	return v1UsersResponse(c, users)
}
