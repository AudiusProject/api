package utils

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Extract the real IP address from the request
// Prioritize X-Forwarded-For, X-Real-IP, and fall back to direct IP
func GetIP(c *fiber.Ctx) string {
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	if xri := c.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	return c.IP()
}
