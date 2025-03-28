package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"bridgerton.audius.co/queries"
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

	app.Get("/", app.Home)
	app.Get("/hello/:name", app.SayHello)
	app.Get("/v2/users/:handle", app.GetUser)
	return app
}

type ApiServer struct {
	*fiber.App
	conn    *pgx.Conn
	queries *queries.Queries
}

func (app *ApiServer) Home(c *fiber.Ctx) error {
	return c.SendString("OK Fiber 4")
}

func (app *ApiServer) SayHello(c *fiber.Ctx) error {
	return c.SendString("hello " + c.Params("name"))
}

func (app *ApiServer) GetUser(c *fiber.Ctx) error {
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
	return c.JSON(users[0])
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
