package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendRelayResponsePreventsResponseTransformation(t *testing.T) {
	app := fiber.New()
	app.Post("/relay-response", func(c *fiber.Ctx) error {
		return sendRelayResponse(c, map[string]interface{}{
			"transactionHash": "0xabc",
			"status":          true,
		})
	})

	req := httptest.NewRequest("POST", "/relay-response", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, res.StatusCode)
	assert.Equal(t, "no-store, no-transform", res.Header.Get(fiber.HeaderCacheControl))
	assert.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, res.Header.Get(fiber.HeaderContentType))
	assert.Equal(t, len(body), int(res.ContentLength))

	var decoded map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "0xabc", decoded["receipt"]["transactionHash"])
	assert.Equal(t, true, decoded["receipt"]["status"])
}
