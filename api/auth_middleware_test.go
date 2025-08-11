package api

import (
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"bridgerton.audius.co/database"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRecoverAuthorityFromSignatureHeaders(t *testing.T) {
	app := testAppWithFixtures(t)
	var wallet string

	// Create a dummy endpoint to test the authMiddleware
	testApp := fiber.New()
	testApp.Get("/", app.authMiddleware, func(c *fiber.Ctx) error {
		wallet = app.getAuthedWallet(c)
		return c.SendStatus(fiber.StatusOK)
	})

	{
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Encoded-Data-Message", "signature:1744763856446")
		req.Header.Set("Encoded-Data-Signature", "0xbb202be3a7f3a0aa22c1458ef6a3f2f8360fb86791c7b137e8562df0707825c11fa1db01096efd2abc5e6613c4d1e8d4ae1e2b993abdd555fe270c1b17bff0d21c")

		_, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, "0x7d273271690538cf855e5b3002a0dd8c154bb060", wallet)
	}

	// Test using query params
	{
		req := httptest.NewRequest("GET", "/?user_data=signature:1744763856446&user_signature=0xbb202be3a7f3a0aa22c1458ef6a3f2f8360fb86791c7b137e8562df0707825c11fa1db01096efd2abc5e6613c4d1e8d4ae1e2b993abdd555fe270c1b17bff0d21c", nil)

		_, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, "0x7d273271690538cf855e5b3002a0dd8c154bb060", wallet)
	}
}

func TestAuthorized(t *testing.T) {
	app := testAppWithFixtures(t)

	// Create a dummy endpoint to test the authMiddleware
	testApp := fiber.New()
	testApp.Get("/", app.resolveMyIdMiddleware, app.authMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	testApp.Get("/account/:wallet", app.resolveMyIdMiddleware, app.authMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Forbidden when not authorized
	req := httptest.NewRequest("GET", "/?user_id=7eP5n", nil)
	// wallet: 0x681c616ae836ceca1effe00bd07f2fdbf9a082bc
	req.Header.Set("Encoded-Data-Message", "signature:1745543704165")
	req.Header.Set("Encoded-Data-Signature", "0x4af765948dccd72026f1059a59c7a6a1172628255d7d387d1590c0fe43961c5908fc6011443805ca0dbd39156300c04dc21bbfa9adce50acea9ad29a7e2fde2a1b")
	res, err := testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, res.StatusCode)

	// Forbidden when grant is revoked
	req = httptest.NewRequest("GET", "/?user_id=7eP5n", nil)
	// wallet: 0xc451c1f8943b575158310552b41230c61844a1c1
	req.Header.Set("Encoded-Data-Message", "signature:1745542789211")
	req.Header.Set("Encoded-Data-Signature", "0xffd5f92c0d253c7222cd407cf3398fac664530ef968bd4435ea698ba1daee1d73353330848b65d212eeeaae9f41e177e49078c4efa1131e5e517090626f6dd961c")
	res, err = testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, res.StatusCode)

	// Authorized when grant is approved
	req = httptest.NewRequest("GET", "/?user_id=7eP5n", nil)
	// wallet: 0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8
	req.Header.Set("Encoded-Data-Message", "signature:1745544459796")
	req.Header.Set("Encoded-Data-Signature", "0x1c9cb405d8437d28ff5596918551f7a45f981e81618d65ee10892313292a8c7a325af002231d115b28ca2d244b082abe1bde4a7d9610f8140d3738a9be5c4fd91b")
	res, err = testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, res.StatusCode)

	// Authorized when own user
	req = httptest.NewRequest("GET", "/?user_id=7eP5n", nil)
	// wallet: 0x7d273271690538cf855e5b3002a0dd8c154bb060
	req.Header.Set("Encoded-Data-Message", "signature:1744763856446")
	req.Header.Set("Encoded-Data-Signature", "0xbb202be3a7f3a0aa22c1458ef6a3f2f8360fb86791c7b137e8562df0707825c11fa1db01096efd2abc5e6613c4d1e8d4ae1e2b993abdd555fe270c1b17bff0d21c")
	res, err = testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, res.StatusCode)

	// Forbidden when not authorized to act on behalf of requested wallet
	req = httptest.NewRequest("GET", "/account/0x111c616ae836ceca1effe00bd07f2fdbf9a082bc", nil)
	// wallet: 0x681c616ae836ceca1effe00bd07f2fdbf9a082bc
	req.Header.Set("Encoded-Data-Message", "signature:1745543704165")
	req.Header.Set("Encoded-Data-Signature", "0x4af765948dccd72026f1059a59c7a6a1172628255d7d387d1590c0fe43961c5908fc6011443805ca0dbd39156300c04dc21bbfa9adce50acea9ad29a7e2fde2a1b")
	res, err = testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, res.StatusCode)

	// Authorized when requesting wallet matches authed wallet
	req = httptest.NewRequest("GET", "/account/0x681c616ae836ceca1effe00bd07f2fdbf9a082bc", nil)
	// wallet: 0x681c616ae836ceca1effe00bd07f2fdbf9a082bc
	req.Header.Set("Encoded-Data-Message", "signature:1745543704165")
	req.Header.Set("Encoded-Data-Signature", "0x4af765948dccd72026f1059a59c7a6a1172628255d7d387d1590c0fe43961c5908fc6011443805ca0dbd39156300c04dc21bbfa9adce50acea9ad29a7e2fde2a1b")
	res, err = testApp.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, res.StatusCode)
}

func TestRequireAuthMiddleware(t *testing.T) {
	app := testAppWithFixtures(t)

	// Create a dummy endpoint to test the requireAuthMiddleware
	testApp := fiber.New()
	testApp.Get("/", app.resolveMyIdMiddleware, app.authMiddleware, app.requireAuthMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Unauthorized when no auth headers
	req1 := httptest.NewRequest("GET", "/", nil)
	res, err := testApp.Test(req1, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, res.StatusCode)
}

func TestWalletCache(t *testing.T) {
	app := emptyTestApp(t)

	// Create a dummy endpoint to test wallet to user ID lookup
	// The important part here is that we are pulling a value from the context
	// and passing it directly to getUserIDFromWallet.
	testApp := fiber.New()
	testApp.Get("/test/get_user_id/:wallet", func(c *fiber.Ctx) error {
		wallet := c.Params("wallet")
		userId, err := app.getUserIDFromWallet(c.Context(), wallet)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"data": userId,
		})
	})

	fixtures := database.FixtureMap{
		"users": make([]map[string]any, 100),
	}

	for i := range 100 {
		fixtures["users"][i] = map[string]any{
			"user_id":   i + 1,
			"handle":    fmt.Sprintf("testuser%d", i+1),
			"handle_lc": fmt.Sprintf("testuser%d", i+1),
			"wallet":    fmt.Sprintf("0x%064x", i+1),
		}
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Test that caching doesn't reuse context params
	for i := range 1000 {
		user := fixtures["users"][i%100]
		wallet := user["wallet"].(string)
		expectedUserId := user["user_id"].(int)

		req := httptest.NewRequest("GET", "/test/get_user_id/"+wallet, nil)
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		assert.NoError(t, err)

		if ok := jsonAssert(t, body, map[string]any{
			"data": expectedUserId,
		}); !ok {
			fmt.Printf("failed on iteration %d\n", i)
			return
		}
	}
}
