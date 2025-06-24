package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/api/searchv1"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

func (app *ApiServer) v1SearchFull(c *fiber.Ctx) error {
	kind := c.Query("kind", "all")

	g := errgroup.Group{}
	var users = []dbv1.FullUser{}
	var tracks = []dbv1.FullTrack{}
	var playlists = []dbv1.FullPlaylist{}
	var albums = []dbv1.FullPlaylist{}

	// users
	g.Go(func() (err error) {
		if kind != "all" && kind != "users" {
			return nil
		}

		users, err = app.searchUsers(c)
		return err
	})

	// tracks
	g.Go(func() (err error) {
		if kind != "all" && kind != "tracks" {
			return nil
		}

		tracks, err = app.searchTracks(c)
		return err
	})

	// playlists
	g.Go(func() (err error) {
		if kind != "all" && kind != "playlists" {
			return nil
		}

		playlists, err = app.searchPlaylists(c)
		return err
	})

	// albums
	g.Go(func() (err error) {
		if kind != "all" && kind != "albums" {
			return nil
		}

		albums, err = app.searchAlbums(c)
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

func (app *ApiServer) searchUsers(c *fiber.Ctx) ([]dbv1.FullUser, error) {
	isTagSearch := strings.Contains(c.Route().Path, "search/tags")
	isFullSearch := strings.Contains(c.Route().Path, "search/full")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)
	myId := app.getMyId(c)

	q := searchv1.UserSearchQuery{
		Query:       c.Query("query"),
		IsVerified:  c.QueryBool("is_verified"),
		IsTagSearch: isTagSearch,
		Genres:      queryMutli(c, "genre"),
		MyID:        myId,
	}

	userIds, err := searchv1.SearchAndPluck(app.esClient, "users", q.DSL(), limit, offset)
	if err != nil {
		return nil, err
	}

	// savings: only personalize results for "full" endpoint
	if !isFullSearch {
		myId = 0
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  userIds,
		MyID: myId,
	})
	return users, err
}

func (app *ApiServer) searchTracks(c *fiber.Ctx) ([]dbv1.FullTrack, error) {
	isTagSearch := strings.Contains(c.Route().Path, "search/tags")
	isFullSearch := strings.Contains(c.Route().Path, "search/full")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)
	myId := app.getMyId(c)

	q := searchv1.TrackSearchQuery{
		MyID:           myId,
		IsTagSearch:    isTagSearch,
		Query:          c.Query("query"),
		Genres:         queryMutli(c, "genre"),
		Moods:          queryMutli(c, "mood"),
		MinBPM:         c.QueryInt("bpm_min"),
		MaxBPM:         c.QueryInt("bpm_max"),
		MusicalKeys:    queryMutli(c, "key"),
		IsDownloadable: c.QueryBool("is_downloadable"),
		IsPurchaseable: c.QueryBool("is_purchaseable"),
		OnlyVerified:   c.QueryBool("only_verified"),
	}

	tracksIds, err := searchv1.SearchAndPluck(app.esClient, "tracks", q.DSL(), limit, offset)
	if err != nil {
		return nil, err
	}

	// savings: only personalize results for "full" endpoint
	if !isFullSearch {
		myId = 0
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  tracksIds,
			MyID: myId,
		},
	})
	return tracks, err
}

func (app *ApiServer) searchPlaylists(c *fiber.Ctx) ([]dbv1.FullPlaylist, error) {
	isTagSearch := strings.Contains(c.Route().Path, "search/tags")
	isFullSearch := strings.Contains(c.Route().Path, "search/full")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)
	myId := app.getMyId(c)

	q := searchv1.PlaylistSearchQuery{
		MyID:         myId,
		IsTagSearch:  isTagSearch,
		Query:        c.Query("query"),
		Genres:       queryMutli(c, "genre"),
		Moods:        queryMutli(c, "mood"),
		OnlyVerified: c.QueryBool("only_verified"),
	}

	playlistsIds, err := searchv1.SearchAndPluck(app.esClient, "playlists", q.DSL(), limit, offset)
	if err != nil {
		return nil, err
	}

	// savings: only personalize results for "full" endpoint
	if !isFullSearch {
		myId = 0
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  playlistsIds,
			MyID: myId,
		},
		OmitTracks: true,
	})
	return playlists, err
}

func (app *ApiServer) searchAlbums(c *fiber.Ctx) ([]dbv1.FullPlaylist, error) {
	isTagSearch := strings.Contains(c.Route().Path, "search/tags")
	isFullSearch := strings.Contains(c.Route().Path, "search/full")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)
	myId := app.getMyId(c)

	q := searchv1.PlaylistSearchQuery{
		MyID:         myId,
		IsTagSearch:  isTagSearch,
		Query:        c.Query("query"),
		Genres:       queryMutli(c, "genre"),
		Moods:        queryMutli(c, "mood"),
		OnlyVerified: c.QueryBool("only_verified"),
		IsAlbum:      true,
	}

	playlistsIds, err := searchv1.SearchAndPluck(app.esClient, "playlists", q.DSL(), limit, offset)
	if err != nil {
		return nil, err
	}

	// savings: only personalize results for "full" endpoint
	if !isFullSearch {
		myId = 0
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  playlistsIds,
			MyID: myId,
		},
		OmitTracks: true,
	})
	return playlists, err
}
