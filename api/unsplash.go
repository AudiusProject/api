package api

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) unsplash(c *fiber.Ctx) error {
	targetURL := fmt.Sprintf(
		"https://api.unsplash.com%s",
		strings.Replace(c.Path(), "/unsplash", "", 1),
	)

	// Copy query parameters from the original request
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Invalid URL: %v", err))
	}
	query := parsedURL.Query()
	for key, values := range c.Queries() {
		query.Set(key, values)
	}
	parsedURL.RawQuery = query.Encode()

	unsplashKeys := app.config.UnsplashKeys
	if len(unsplashKeys) == 0 {
		return fiber.NewError(fiber.StatusInternalServerError, "No Unsplash keys configured")
	}

	triedIndices := make(map[int]bool)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	client := &http.Client{Timeout: 30 * time.Second}
	body := c.Body()

	// Try keys randomly until we find one that works or exhaust all keys
	for len(triedIndices) < len(unsplashKeys) {
		idx := rng.Intn(len(unsplashKeys))
		if triedIndices[idx] {
			continue
		}
		triedIndices[idx] = true

		query.Set("client_id", unsplashKeys[idx])
		parsedURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(c.Context(), c.Method(), parsedURL.String(), bytes.NewReader(body))
		if err != nil {
			continue
		}

		for key, value := range c.Request().Header.All() {
			keyStr := string(key)
			if strings.ToLower(keyStr) != "host" {
				req.Header.Set(keyStr, string(value))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		// If successful, proxy the response
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()

			for key, values := range resp.Header {
				for _, value := range values {
					c.Set(key, value)
				}
			}
			c.Status(resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			return c.Send(body)
		}

		resp.Body.Close()
	}
	// All keys failed
	return fiber.NewError(fiber.StatusBadGateway, "All Unsplash keys failed")
}
