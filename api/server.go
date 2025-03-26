package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	v2 "bridgerton.audius.co/api/v2"
	"bridgerton.audius.co/queries"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	_ "github.com/joho/godotenv/autoload"
)

func NewApiServer() *ApiServer {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	dbUrl := os.Getenv("discoveryDbUrl")
	conn, err := pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		slog.Warn("db connect failed", "err", err)
	}

	queries := queries.New(conn)
	as := &ApiServer{
		echo.New(),
		conn,
		queries,
	}
	as.Debug = true
	as.HideBanner = true
	as.HTTPErrorHandler = as.errHandler

	// Add logging middleware
	as.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)
			status := c.Response().Status

			entry := slog.Info
			if status >= 500 {
				entry = slog.Error
			}

			entry("request completed",
				"method", c.Request().Method,
				"path", c.Path(),
				"status", status,
				"duration", duration.Milliseconds(),
			)

			return err
		}
	})

	// Register routes
	as.GET("/", v2.Home)

	// Register user routes
	userHandler := v2.NewUserHandler(queries)
	as.GET("/v2/users/:handle", userHandler.GetUser)

	return as
}

type ApiServer struct {
	*echo.Echo
	conn    *pgx.Conn
	queries *queries.Queries
}

func (as *ApiServer) errHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	if err == pgx.ErrNoRows {
		code = 404
	} else if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}
	c.Logger().Error(err)
	c.JSON(code, map[string]any{
		"code":  code,
		"error": err.Error(),
	})
}

func (as *ApiServer) Serve() {
	as.Logger.Fatal(as.Start(":1323"))
}
