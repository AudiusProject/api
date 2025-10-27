package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/api/searchv1"
	"github.com/gofiber/fiber/v2"
)

type GetPlaylistsTopParams struct {
	Limit  int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Mood   string `query:"mood"`
	Type   string `query:"type" default:"playlist" validate:"oneof=playlist album"`
}

func (app *ApiServer) v1PlaylistsTop(c *fiber.Ctx) error {
	params := GetPlaylistsTopParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	moods := []string{}
	if params.Mood != "" {
		// Working around a quirk with mood indexing: track moods begin with a capital letter and must match exactly.
		capitalizedMood := strings.ToUpper(string(params.Mood[0])) + strings.ToLower(params.Mood[1:])
		moods = []string{capitalizedMood}
	}

	q := searchv1.PlaylistSearchQuery{
		IsTagSearch: false,
		Moods:       moods,
		SortMethod:  "popular",
	}

	playlistsIds, err := searchv1.SearchAndPluck(app.esClient, "playlists", q.DSL(), params.Limit, params.Offset)
	if err != nil {
		return err
	}

	myId := int32(0)
	// only personalize results for full endpoint
	if app.getIsFull(c) {
		myId = app.getMyId(c)
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  playlistsIds,
			MyID: myId,
		},
		OmitTracks: true,
	})

	return v1PlaylistsResponse(c, playlists)
}
