package api

import (
	"fmt"
	"math"
	"math/rand"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UnclaimedId(c *fiber.Ctx, tableName, idField string, lowerBound, upperBound int) error {

	sql := fmt.Sprintf(`select true from %s where %s = $1`, tableName, idField)

	freeId := 0
	for i := 0; i < 50; i++ {
		randomId := lowerBound + rand.Intn(upperBound-lowerBound)
		isTaken := false
		err := app.pool.QueryRow(c.Context(), sql, randomId).Scan(&isTaken)
		if err == pgx.ErrNoRows {
			freeId = randomId
			break
		}
	}

	if freeId == 0 {
		return fiber.NewError(500, "unable to find unclaimed id")
	}

	id, _ := trashid.EncodeHashId(freeId)
	return c.JSON(fiber.Map{
		"data": id,
	})
}

func (app *ApiServer) v1UsersUnclaimedId(c *fiber.Ctx) error {
	// max for reward specifier id
	return app.v1UnclaimedId(c, "users", "user_id", 3_000_000, 999_999_999)
}

func (app *ApiServer) v1TracksUnclaimedId(c *fiber.Ctx) error {
	return app.v1UnclaimedId(c, "tracks", "track_id", 2_000_000, math.MaxInt32)
}

func (app *ApiServer) v1PlaylistsUnclaimedId(c *fiber.Ctx) error {
	return app.v1UnclaimedId(c, "tracks", "track_id", 400_000, math.MaxInt32)
}

func (app *ApiServer) v1CommentsUnclaimedId(c *fiber.Ctx) error {
	return app.v1UnclaimedId(c, "comments", "comment_id", 4_000_000, math.MaxInt32)
}

func (app *ApiServer) v1EventsUnclaimedId(c *fiber.Ctx) error {
	return app.v1UnclaimedId(c, "events", "event_id", 1, math.MaxInt32)
}
