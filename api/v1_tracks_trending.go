package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetTrendingTracksParams struct {
	Limit  int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=200"`
	Time   string `query:"time" default:"week" validate:"oneof=week month allTime"`
	Genre  string `query:"genre" default:""`
}

func (app *ApiServer) v1TracksTrending(c *fiber.Ctx) error {
	var params = GetTrendingTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	trackIds, err := app.getTrendingIds(
		c,
		params.Time,
		params.Genre,
		params.Limit,
		params.Offset,
	)

	if err != nil {
		return err
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          trackIds,
			MyID:         myId,
			AuthedWallet: app.tryGetAuthedWallet(c),
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
