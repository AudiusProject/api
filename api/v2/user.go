package v2

import (
	"bridgerton.audius.co/queries"
	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	queries *queries.Queries
}

func NewUserHandler(queries *queries.Queries) *UserHandler {
	return &UserHandler{queries: queries}
}

func (h *UserHandler) GetUser(c echo.Context) error {
	handle := c.Param("handle")
	user, err := h.queries.GetUserByHandle(c.Request().Context(), handle)
	if err != nil {
		return err
	}
	return c.JSON(200, user)
}
