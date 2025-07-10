package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/test-go/testify/assert"
)

func TestGetIP_XForwardedFor(t *testing.T) {
	app := fiber.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.RemoteAddr = "127.0.0.1:12345"

	var result string
	app.Get("/", func(c *fiber.Ctx) error {
		result = GetIP(c)
		return c.SendString("ok")
	})

	app.Test(req)
	assert.Equal(t, "192.168.1.1", result)
}

func TestGetIP_XRealIP(t *testing.T) {
	app := fiber.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1"

	var result string
	app.Get("/", func(c *fiber.Ctx) error {
		result = GetIP(c)
		return c.SendString("ok")
	})

	app.Test(req)
	assert.Equal(t, "10.0.0.1", result)
}

func TestGetIP_DirectIP(t *testing.T) {
	app := fiber.New()
	req := httptest.NewRequest("GET", "/", nil)

	var result string
	app.Get("/", func(c *fiber.Ctx) error {
		result = GetIP(c)
		return c.SendString("ok")
	})

	app.Test(req)
	assert.Equal(t, "0.0.0.0", result)
}

func TestGetIP_InvalidXForwardedFor(t *testing.T) {
	app := fiber.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "invalid-ip")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1"

	var result string
	app.Get("/", func(c *fiber.Ctx) error {
		result = GetIP(c)
		return c.SendString("ok")
	})

	app.Test(req)
	assert.Equal(t, "10.0.0.1", result)
}

func TestGetIP_MultipleXForwardedFor(t *testing.T) {
	app := fiber.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1"

	var result string
	app.Get("/", func(c *fiber.Ctx) error {
		result = GetIP(c)
		return c.SendString("ok")
	})

	app.Test(req)
	assert.Equal(t, "192.168.1.1", result)
}
