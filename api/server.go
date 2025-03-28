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

	var userIds []int32
	for _, b := range c.Request().URI().QueryArgs().PeekMulti("id") {
		if id, err := trashid.DecodeHashId(string(b)); err == nil {
			userIds = append(userIds, int32(id))
		}
	}
	if len(userIds) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": 400,
			"error":  "id query param required",
		})
	}

	users, err := app.queries.GetUsers(c.Context(), queries.GetUsersParams{
		MyID: int32(myId),
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	// todo: need to do id encode (intermediate query layer above sqlc)
	return c.JSON(fiber.Map{
		"data": users,
	})
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
		ID string `json:"id"`
	}{
		user,
		trashid.MustEncodeHashID(int(user.UserID)),
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

func (as *ApiServer) Serve() {
	as.Listen(":1323")
}
