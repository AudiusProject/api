package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	adapter "github.com/axiomhq/axiom-go/adapters/zap"
	"github.com/axiomhq/axiom-go/axiom"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	DbUrl        string
	AxiomToken   string
	AxiomDataset string
}

func InitLogger(config Config) *zap.Logger {
	// stdout core
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewJSONEncoder(encoderConfig)
	stdoutCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	var core zapcore.Core = stdoutCore

	// axiom core, if token and dataset are provided
	if config.AxiomToken != "" && config.AxiomDataset != "" {
		axiomAdapter, err := adapter.New(
			adapter.SetClientOptions(
				axiom.SetAPITokenConfig(config.AxiomToken),
				axiom.SetOrganizationID("audius-Lu52"),
			),
			adapter.SetDataset(config.AxiomDataset),
		)
		if err != nil {
			panic(err)
		}

		core = zapcore.NewTee(stdoutCore, axiomAdapter)
	}

	logger := zap.New(core)
	return logger
}

func NewApiServer(config Config) *ApiServer {
	logger := InitLogger(config)

	conn, err := pgx.Connect(context.Background(), config.DbUrl)
	if err != nil {
		logger.Error("db connect failed", zap.Error(err))
	}

	app := &ApiServer{
		fiber.New(fiber.Config{
			ErrorHandler: errorHandler(logger),
		}),
		conn,
		queries.New(conn),
		logger,
	}

	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
	}))

	app.Get("/", app.home)
	app.Get("/v2/users/:handle", app.getUser)
	app.Get("/v1/full/users", app.getUsers)
	app.Get("/v1/full/tracks", app.getTracks)

	// gracefully handle 404
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"code":  http.StatusNotFound,
			"error": "Route not found",
		})
	})

	return app
}

type ApiServer struct {
	*fiber.App
	conn    *pgx.Conn
	queries *queries.Queries
	logger  *zap.Logger
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

func errorHandler(logger *zap.Logger) func(*fiber.Ctx, error) error {
	return func(ctx *fiber.Ctx, err error) error {
		code := http.StatusInternalServerError
		if err == pgx.ErrNoRows {
			code = http.StatusNotFound
		}

		return ctx.Status(code).JSON(&fiber.Map{
			"code":  code,
			"error": err.Error(),
		})
	}
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
	flushTicker := time.NewTicker(time.Second * 15)
	go func() {
		for range flushTicker.C {
			as.logger.Sync()
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		flushTicker.Stop()
		as.Shutdown()
		as.conn.Close(context.Background())
		as.logger.Sync()
	}()

	if err := as.Listen(":1323"); err != nil && err != http.ErrServerClosed {
		as.logger.Fatal("Failed to start server", zap.Error(err))
	}
}
