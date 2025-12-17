package api

import (
	"math/rand"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) contentNodes(c *fiber.Ctx) error {
	var contentNodes []fiber.Map
	for _, node := range app.validators.GetNodes() {
		if node.ServiceType == "content-node" {
			contentNodes = append(contentNodes, fiber.Map{
				"spID":                node.Id,
				"owner":               node.Owner,
				"endpoint":            node.Endpoint,
				"delegateOwnerWallet": node.DelegateWallet,
				"type":                node.ServiceType,
				"blockNumber":         node.BlockNumber,
			})
		}
	}
	rand.Shuffle(len(contentNodes), func(i, j int) {
		contentNodes[i], contentNodes[j] = contentNodes[j], contentNodes[i]
	})

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
