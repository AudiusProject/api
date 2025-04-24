package api

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/valyala/fasthttp"
)

type roundrobin struct {
	sync.Mutex

	current int
	pool    []string
}

// this method will return a string of addr server from list server.
func (r *roundrobin) get() string {
	r.Lock()
	defer r.Unlock()

	if len(r.pool) == 0 {
		return ""
	}

	if r.current >= len(r.pool) {
		r.current %= len(r.pool)
	}

	result := r.pool[r.current]
	r.current++
	return result
}

// Modified BalancerForward to add upstream server to fiber context
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler {
	if len(servers) == 0 {
		return func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
	}

	r := &roundrobin{
		current: 0,
		pool:    servers,
	}
	return func(c *fiber.Ctx) error {
		server := r.get()
		if !strings.HasPrefix(server, "http") {
			server = "http://" + server
		}
		c.Locals("upstream", server)
		c.Request().Header.Add("X-Real-IP", c.IP())
		return proxy.Do(c, server+c.OriginalURL(), clients...)
	}
}
