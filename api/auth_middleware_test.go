package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRecoverAuthorityFromSignatureHeaders(t *testing.T) {
	app := fixturesTestApp(t)

	var userId int32
	var wallet string

	// Create a dummy endpoint to test the authMiddleware
	testApp := fiber.New()
	testApp.Get("/", app.authMiddleware, func(c *fiber.Ctx) error {
		userId, wallet = app.getAuthedUserId(c), app.getAuthedWallet(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Encoded-Data-Message", "signature:1744763856446")
	req.Header.Set("Encoded-Data-Signature", "0xbb202be3a7f3a0aa22c1458ef6a3f2f8360fb86791c7b137e8562df0707825c11fa1db01096efd2abc5e6613c4d1e8d4ae1e2b993abdd555fe270c1b17bff0d21c")

	_, err := testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), userId)
	assert.Equal(t, "0x7d273271690538cf855e5b3002a0dd8c154bb060", wallet)
}
