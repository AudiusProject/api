package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) contentNodes(c *fiber.Ctx) error {
	var contentNodes []fiber.Map
	for _, node := range app.validators.GetNodes() {
		if node.ServiceType == "content-node" {
			contentNodes = append(contentNodes, fiber.Map{
				"id":             node.Id,
				"owner":          node.Owner,
				"endpoint":       node.Endpoint,
				"delegateWallet": node.DelegateWallet,
				"serviceType":    node.ServiceType,
				"registeredAt":   node.RegisteredAt,
			})
		}
	}

	if strings.HasSuffix(c.Path(), "/verbose") {
		return c.JSON(fiber.Map{
			"data": contentNodes,
		})
	}

	// Return just URLs as strings
	urls := make([]string, len(contentNodes))
	for i, node := range contentNodes {
		urls[i] = node["endpoint"].(string)
	}

	return c.JSON(fiber.Map{
		"data": urls,
	})
}
