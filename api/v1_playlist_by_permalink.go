package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type v1PlaylistsByPermlinkRouteParams struct {
	Handle string `params:"handle"`
	Slug   string `params:"slug"`
}

func (app *ApiServer) v1PlaylistsByPermalink(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	params := &v1PlaylistsByPermlinkRouteParams{}
	err := c.ParamsParser(params)
	if err != nil {
		return err
	}

	ids, err := app.queries.GetPlaylistIdsByPermalink(c.Context(), dbv1.GetPlaylistIdsByPermalinkParams{
		Handles:    []string{params.Handle},
		Slugs:      []string{params.Slug},
		Permalinks: []string{"/" + params.Handle + "/playlist/" + params.Slug},
	})
	if err != nil {
		return err
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			MyID: myId,
			Ids:  ids,
		},
	})
	if err != nil {
		return err
	}

	return v1PlaylistsResponse(c, playlists)
}
