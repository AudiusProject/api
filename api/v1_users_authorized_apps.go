package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

type GetUsersAuthorizedAppsQueryParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type UserAuthorizedApp struct {
	Address        string     `json:"address"`
	Name           string     `json:"name"`
	Description    *string    `json:"description"`
	ImageUrl       *string    `json:"image_url"`
	GrantorUserID  string     `json:"grantor_user_id"`
	GrantCreatedAt *time.Time `json:"grant_created_at"`
	GrantUpdatedAt *time.Time `json:"grant_updated_at"`
}

func (app *ApiServer) v1UsersAuthorizedApps(c *fiber.Ctx) error {
	queryParams := GetUsersAuthorizedAppsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	// Get authorized apps for the user
	rows, err := app.queries.GetDeveloperAppsWithGrants(c.Context(), dbv1.GetDeveloperAppsWithGrantsParams{
		UserID: trashid.HashId(app.getUserId(c)),
	})
	if err != nil {
		return err
	}

	// Convert to response format and apply pagination
	var authorizedApps []UserAuthorizedApp
	start := queryParams.Offset
	end := queryParams.Offset + queryParams.Limit

	authorizedApps = make([]UserAuthorizedApp, 0, queryParams.Limit)
	for i := start; i < end && i < len(rows); i++ {
		row := rows[i]
		grantorUserID, err := trashid.EncodeHashId(int(row.GrantorUserID))
		if err != nil {
			return err
		}

		app := UserAuthorizedApp{
			Address:        row.Address,
			Name:           row.Name,
			GrantorUserID:  grantorUserID,
			GrantCreatedAt: row.GrantCreatedAt,
			GrantUpdatedAt: row.GrantUpdatedAt,
		}
		if row.Description.Valid {
			app.Description = &row.Description.String
		}
		if row.ImageUrl.Valid {
			app.ImageUrl = &row.ImageUrl.String
		}
		authorizedApps = append(authorizedApps, app)
	}

	return c.JSON(fiber.Map{
		"data": authorizedApps,
	})
}
