package api

import (
	"api.audius.co/rendezvous"
	"github.com/gofiber/fiber/v2"
)

// intakes a cid and returns a list of nodes that have the cid based on the rendezvous algorithm
// also accepts a number of nodes to return
func (app *ApiServer) getRendezvous(c *fiber.Ctx) error {
	cid := c.Params("cid")
	if cid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cid is required",
		})
	}

	nodes := app.validators.GetNodes()
	endpoints := make([]string, len(nodes))
	for i, node := range nodes {
		endpoints[i] = node.Endpoint
	}
	first, rest := rendezvous.GlobalHasher.ReplicaSet3(cid)

	return c.JSON(fiber.Map{
		"first":      first,
		"rest":       rest,
		"validators": endpoints,
	})
}
