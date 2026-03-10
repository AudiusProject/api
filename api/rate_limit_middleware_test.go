package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO api_keys (api_key, api_secret, rps, rpm)
		VALUES ('0xrate-test-key', NULL, 1, 2)
		ON CONFLICT (api_key) DO UPDATE SET rps = 1, rpm = 2
	`)
	assert.NoError(t, err)

	logger := logging.NewZapLogger(config.Config{}).With()
	rlm := NewRateLimitMiddleware(logger, pool)

	testApp := fiber.New()
	testApp.Use(rlm.Middleware(nil))
	testApp.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/test?api_key=0xrate-test-key", nil)
	res1, err := testApp.Test(req1, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, res1.StatusCode, "first request should succeed")

	// Second request within same second should be rate limited (rps=1)
	req2 := httptest.NewRequest("GET", "/test?api_key=0xrate-test-key", nil)
	res2, err := testApp.Test(req2, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, res2.StatusCode, "second request should be rate limited")
}

func TestRateLimitMiddleware_NormalizesApiKeyWithout0xPrefix(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO api_keys (api_key, api_secret, rps, rpm)
		VALUES ('0x6c1ef2e9c33e2ba1c0d352e41e06e8a3c7721c6f', NULL, 1, 1000)
		ON CONFLICT (api_key) DO UPDATE SET rps = 1, rpm = 1000
	`)
	assert.NoError(t, err)

	logger := logging.NewZapLogger(config.Config{}).With()
	rlm := NewRateLimitMiddleware(logger, pool)

	testApp := fiber.New()
	testApp.Use(rlm.Middleware(nil))
	testApp.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Request omits 0x prefix but should still match stored 0x api_key row.
	req1 := httptest.NewRequest("GET", "/test?api_key=6c1ef2e9c33e2ba1c0d352e41e06e8a3c7721c6f", nil)
	res1, err := testApp.Test(req1, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, res1.StatusCode, "first request should succeed")

	// If normalization works, this hits app rps=1 and should be limited.
	req2 := httptest.NewRequest("GET", "/test?api_key=6c1ef2e9c33e2ba1c0d352e41e06e8a3c7721c6f", nil)
	res2, err := testApp.Test(req2, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, res2.StatusCode, "second request should be rate limited")
}
