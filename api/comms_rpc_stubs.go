package api

import "github.com/gofiber/fiber/v2"

func (app *ApiServer) getRpcBulkStub(c *fiber.Ctx) error {
	app.logger.Warn("Received unexpected request to stubbed endpoint /rpc/bulk")
	return c.JSON([]any{})
}

func (app *ApiServer) postRpcReceiveStub(c *fiber.Ctx) error {
	app.logger.Warn("Received unexpected request to stubbed endpoint /rpc/receive")
	return c.SendString("OK")
}
