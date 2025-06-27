package api

import (
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1UsersNowPlaying implements GET /users/:userId/now-playing.
// It returns the most recent play made by the user if the track
// should still be playing (i.e. created_at + duration > current time).
// Otherwise it returns null.
func (app *ApiServer) v1UsersNowPlaying(c *fiber.Ctx) error {
	sql := `
        SELECT plays.play_item_id, plays.created_at, tracks.duration, tracks.title
        FROM plays
        JOIN tracks ON plays.play_item_id = tracks.track_id
        WHERE plays.user_id = @userId
        ORDER BY plays.created_at DESC
        LIMIT 1
    `

	type nowPlayingRow struct {
		TrackID   int32     `db:"play_item_id"`
		CreatedAt time.Time `db:"created_at"`
		Duration  int32     `db:"duration"`
		Title     string    `db:"title"`
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
	})
	if err != nil {
		return err
	}

	np, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nowPlayingRow])
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.JSON(fiber.Map{
				"data": nil,
			})
		}
		return err
	}

	// Validate that the track is still within its playback window.
	// Add a 10 second buffer to account for the track stopping and the next
	// track getting indexed.
	endTime := np.CreatedAt.Add(time.Duration(np.Duration) * time.Second)
	if endTime.Before(time.Now().Add(10 * time.Second)) {
		return c.JSON(fiber.Map{
			"data": nil,
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"title": np.Title,
			"id":    trashid.MustEncodeHashID(int(np.TrackID)),
		},
	})
}
