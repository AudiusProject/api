package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) getChatWebsocket(c *fiber.Ctx) error {
	wallet := app.getAuthedWallet(c)
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}
	return websocket.New(func(conn *websocket.Conn) {
		// Register the websocket connection
		app.commsRpcProcessor.RegisterWebsocket(int32(userId), conn)
	})(c)
}
