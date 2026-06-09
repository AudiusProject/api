package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetUsersCollaborationInvitesParams struct {
	Status string `query:"status" default:"" validate:"omitempty,oneof=pending accepted rejected"`
}

// v1UsersCollaborationInvites lists the collaborator credits a user has been
// offered. Defaults to all; pass ?status=pending to fetch just the actionable
// invites the user can accept/decline.
func (app *ApiServer) v1UsersCollaborationInvites(c *fiber.Ctx) error {
	params := GetUsersCollaborationInvitesParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	status := pgtype.Text{}
	if params.Status != "" {
		status = pgtype.Text{String: params.Status, Valid: true}
	}

	invites, err := app.queries.FullTrackCollaboratorInvites(c.Context(), dbv1.GetTrackCollaboratorInvitesForUserParams{
		UserID: app.getUserId(c),
		Status: status,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": invites,
	})
}
