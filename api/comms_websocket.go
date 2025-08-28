package api

import (
	"github.com/gofiber/contrib/websocket"
)

func (app *ApiServer) getChatWebsocket(conn *websocket.Conn) {
	userId := int32(conn.Locals("websocketUserId").(int))

	app.commsRpcProcessor.RegisterWebsocket(userId, conn)
}
