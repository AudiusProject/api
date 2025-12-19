package api

import (
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"

	"api.audius.co/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func archiveProxy(c *fiber.Ctx) error {
	// Handle preflight
	if c.Method() == fiber.MethodOptions {
		c.Response().Header.Set("Access-Control-Allow-Origin", "*")
		c.Response().Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Response().Header.Set("Access-Control-Allow-Headers", "*")
		return c.Status(fiber.StatusNoContent).Send(nil)
	}
	randIdx := rand.Intn(len(config.Cfg.ArchiverNodes))
	upstream, _ := url.Parse(config.Cfg.ArchiverNodes[randIdx])
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		return nil
	}
	c.Set("Access-Control-Allow-Origin", "*")
	return adaptor.HTTPHandler(proxy)(c)
}
