package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1TracksTrending(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	trackIds, err := app.getTrendingIds(
		c,
		c.Query("time", "week"),
		c.Query("genre", ""),
		c.QueryInt("limit", 100),
		c.QueryInt("offset", 0),
	)

	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
