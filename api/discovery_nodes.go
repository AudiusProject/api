package api

import (
	"math/rand"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) discoveryNodes(c *fiber.Ctx) error {
	var discoveryNodes []fiber.Map
	for _, node := range app.validators.GetNodes() {
		if node.ServiceType == "discovery-node" {
			discoveryNodes = append(discoveryNodes, fiber.Map{
				"spID":                node.Id,
				"owner":               node.Owner,
				"endpoint":            node.Endpoint,
				"delegateOwnerWallet": node.DelegateWallet,
				"type":                node.ServiceType,
				"blockNumber":         node.BlockNumber,
			})
		}
	}
	rand.Shuffle(len(discoveryNodes), func(i, j int) {
		discoveryNodes[i], discoveryNodes[j] = discoveryNodes[j], discoveryNodes[i]
	})

	if strings.HasSuffix(c.Path(), "/verbose") {
		return c.JSON(fiber.Map{
			"data": discoveryNodes,
		})
	}

	// Return just URLs as strings
	urls := make([]string, len(discoveryNodes))
	for i, node := range discoveryNodes {
		urls[i] = node["endpoint"].(string)
	}

	return c.JSON(fiber.Map{
		"data": urls,
	})
}
