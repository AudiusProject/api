package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/searcher"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

func (app *ApiServer) v1SearchAutocomplete(c *fiber.Ctx) error {
	// queryMap := c.Queries()

	query := c.Query("query")
	genres := queryMutli(c, "genre")
	moods := queryMutli(c, "mood")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)

	myId := app.getMyId(c)

	g := errgroup.Group{}
	var users []dbv1.FullUser
	var tracks []dbv1.FullTrack
	var playlists []dbv1.FullPlaylist
	var albums []dbv1.FullPlaylist

	// users
	g.Go(func() error {
		q := searcher.UserSearchQuery{
			Query:      query,
			IsVerified: c.QueryBool("is_verified"),
		}

		dsl := searcher.BuildFunctionScoreDSL("follower_count", q.Map())
		userIds, err := searcher.SearchAndPluck(app.esClient, "users", dsl, limit, offset)
		if err != nil {
			return err
		}

		users, err = app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
			Ids:  userIds,
			MyID: myId,
		})
		return err
	})

	// tracks
	g.Go(func() error {
		q := searcher.TrackSearchQuery{
			Query:          query,
			Genres:         genres,
			Moods:          moods,
			MinBPM:         c.QueryInt("bpm_min"),
			MaxBPM:         c.QueryInt("bpm_max"),
			MusicalKeys:    queryMutli(c, "key"),
			IsDownloadable: c.QueryBool("is_downloadable"),
			IsPurchaseable: c.QueryBool("is_purchaseable"),
			// todo: includePurchaseable
			// todo: tags
		}

		dsl := searcher.BuildFunctionScoreDSL("repost_count", q.Map())

		tracksIds, err := searcher.SearchAndPluck(app.esClient, "tracks", dsl, limit, offset)
		if err != nil {
			return err
		}

		tracks, err = app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
			GetTracksParams: dbv1.GetTracksParams{
				Ids:  tracksIds,
				MyID: myId,
			},
		})
		return err
	})

	// playlists
	g.Go(func() error {
		q := searcher.PlaylistSearchQuery{
			Query: query,
		}

		dsl := searcher.BuildFunctionScoreDSL("repost_count", q.Map())

		playlistsIds, err := searcher.SearchAndPluck(app.esClient, "playlists", dsl, limit, offset)
		if err != nil {
			return err
		}

		playlists, err = app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
			GetPlaylistsParams: dbv1.GetPlaylistsParams{
				Ids:  playlistsIds,
				MyID: myId,

				// todo: playlist needs to support modds + genres + tags query
			},
		})
		return err
	})

	// albums
	g.Go(func() error {
		q := searcher.PlaylistSearchQuery{
			Query:   query,
			IsAlbum: true,
		}

		dsl := searcher.BuildFunctionScoreDSL("repost_count", q.Map())

		playlistsIds, err := searcher.SearchAndPluck(app.esClient, "playlists", dsl, limit, offset)
		if err != nil {
			return err
		}

		albums, err = app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
			GetPlaylistsParams: dbv1.GetPlaylistsParams{
				Ids:  playlistsIds,
				MyID: myId,
			},
		})
		return err
	})

	err := g.Wait()
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"users":     users,
			"tracks":    tracks,
			"playlists": playlists,
			"albums":    albums,

			// todos
			"saved_albums":    []any{},
			"saved_users":     []any{},
			"saved_tracks":    []any{},
			"saved_playlists": []any{},
		},
	})
}
