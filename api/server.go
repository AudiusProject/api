package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func NewApiServer() *ApiServer {
	as := &ApiServer{
		echo.New(),
	}
	as.HideBanner = true
	as.GET("/", as.Home)
	as.GET("/hello/:name", as.SayHello)
	return as
}

type ApiServer struct {
	*echo.Echo
}

func (as *ApiServer) Home(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func (as *ApiServer) SayHello(c echo.Context) error {
	return c.String(http.StatusOK, "hello "+c.Param("name"))
}

func (as *ApiServer) Serve() {
	as.Logger.Fatal(as.Start(":1323"))
}
