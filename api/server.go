package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/jackc/pgx/v5"
)

type Config struct {
	DBURL string
}

func NewApiServer(config Config) *ApiServer {

	conn, err := pgx.Connect(context.Background(), config.DBURL)
	if err != nil {
		slog.Warn("db connect failed", "err", err)
	}

	app := &ApiServer{
		fiber.New(fiber.Config{
			ErrorHandler: errorHandler,
		}),
		conn,
		queries.New(conn),
	}

	// todo: structured request logger
	app.Use(logger.New())

	app.Get("/", app.home)
	app.Get("/v2/users/:handle", app.getUser)
	app.Get("/v1/full/users", app.getUsers)
	app.Get("/v1/full/tracks", app.getTracks)
	return app
}

type ApiServer struct {
	*fiber.App
	conn    *pgx.Conn
	queries *queries.Queries
}

func (app *ApiServer) home(c *fiber.Ctx) error {
	return c.SendString("OK")
}

func (app *ApiServer) getUsers(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": 400,
			"error":  "id query param required",
		})
	}

	users, err := app.queries.GetUsers(c.Context(), queries.GetUsersParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	// todo: need to do id encode (intermediate query layer above sqlc)
	return c.JSON(fiber.Map{
		"data": users,
	})
}

func (app *ApiServer) getTracks(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	tracks, err := app.queries.GetTracks(c.Context(), queries.GetTracksParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}
	return c.JSON(tracks)
}

func (app *ApiServer) getUser(c *fiber.Ctx) error {
	// todo: hashid decode crap
	myId, _ := strconv.Atoi(c.Query("user_id"))

	handle := c.Params("handle")
	users, err := app.queries.GetUsers(c.Context(), queries.GetUsersParams{
		MyID:   int32(myId),
		Handle: handle,
	})
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return pgx.ErrNoRows
	}

	// add hash id :vomitemoji:
	user := users[0]

	withHashId := struct {
		queries.GetUsersRow
		// hash id
		ID string `json:"id"`
		// todo: computed image fields
		// todo: computed wallet balance stuff
	}{
		GetUsersRow: user,
		ID:          trashid.MustEncodeHashID(int(user.UserID)),
	}

	return c.JSON(fiber.Map{
		"data": withHashId,
	})
}

func errorHandler(ctx *fiber.Ctx, err error) error {

	code := http.StatusInternalServerError
	if err == pgx.ErrNoRows {
		code = 404
	}

	if code >= 500 {
		slog.Error(ctx.OriginalURL(), "err", err)
	}

	return ctx.Status(code).JSON(&fiber.Map{
		"code":  code,
		"error": err.Error(),
	})
}

func decodeIdList(c *fiber.Ctx) []int32 {
	var ids []int32
	for _, b := range c.Request().URI().QueryArgs().PeekMulti("id") {
		if id, err := trashid.DecodeHashId(string(b)); err == nil {
			ids = append(ids, int32(id))
		}
	}
	return ids
}

func (as *ApiServer) Serve() {
	as.Listen(":1323")
}
