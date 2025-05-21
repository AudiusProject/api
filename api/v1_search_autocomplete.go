package api

import (
	"fmt"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/searcher"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

func (app *ApiServer) v1SearchAutocomplete(c *fiber.Ctx) error {
	query := c.Query("query")
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)

	myId := app.getMyId(c)

	g := errgroup.Group{}
	var users []dbv1.FullUser
	var tracks []dbv1.FullTrack
	var playlists []dbv1.FullPlaylist

	// users
	g.Go(func() error {

		dsl := fmt.Sprintf(`{
			"query": {
				"function_score": {
					"simple_query_string": {
						"query": %q,
						"default_operator": "AND"
					}
				},
				"boost_mode": "sum",
				"score_mode": "sum",
				"functions": [
					{
						"field_value_factor": {
							"field": "follower_count",
							"factor": 1,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}`, query+"*")

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
		dsl := fmt.Sprintf(`{
			"query": {
				"simple_query_string": {
					"query": %q,
					"default_operator": "AND"
				}
			}
		}`, query+"*")

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
		dsl := fmt.Sprintf(`{
			"query": {
				"simple_query_string": {
					"query": %q,
					"default_operator": "AND"
				}
			}
		}`, query+"*")

		playlistsIds, err := searcher.SearchAndPluck(app.esClient, "playlists", dsl, limit, offset)
		if err != nil {
			return err
		}

		playlists, err = app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
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

			// todos
			"albums":          []any{},
			"saved_albums":    []any{},
			"saved_users":     []any{},
			"saved_tracks":    []any{},
			"saved_playlists": []any{},
		},
	})
}
