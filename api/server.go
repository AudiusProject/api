package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"bridgerton.audius.co/queries"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	_ "github.com/joho/godotenv/autoload"
)

func NewApiServer() *ApiServer {
	dbUrl := os.Getenv("discoveryDbUrl")
	conn, err := pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		slog.Warn("db connect failed", "err", err)
	}

	as := &ApiServer{
		echo.New(),
		conn,
		queries.New(conn),
	}
	as.Debug = true
	as.HideBanner = true
	as.HTTPErrorHandler = as.errHandler
	as.GET("/", as.Home)
	as.GET("/hello/:name", as.SayHello)
	as.GET("/v2/users/:handle", as.GetUser)
	return as
}

type ApiServer struct {
	*echo.Echo
	conn    *pgx.Conn
	queries *queries.Queries
}

func (as *ApiServer) Home(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func (as *ApiServer) SayHello(c echo.Context) error {
	return c.String(http.StatusOK, "hello "+c.Param("name"))
}

func (as *ApiServer) GetUser(c echo.Context) error {
	handle := c.Param("handle")
	user, err := as.queries.GetUserByHandle(c.Request().Context(), handle)
	if err != nil {
		return err
	}
	return c.JSON(200, user)
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
