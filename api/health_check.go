package api

import (
	"github.com/gofiber/fiber/v2"
)

type contentNode struct {
	// camelCase to maintain backwards compatibility
	DelegateOwnerWallet string `json:"delegateOwnerWallet"`
	Endpoint            string `json:"endpoint"`
}

type networkInfo struct {
	ContentNodes []contentNode `json:"content_nodes"`
}

type healthCheckResponse struct {
	Network networkInfo `json:"network"`
}

func (app *ApiServer) healthCheck(c *fiber.Ctx) error {
	healthyNodes := app.contentNodeMonitor.GetContentNodes()
	// Convert config.Node to contentNode
	contentNodes := make([]contentNode, len(healthyNodes))
	for i, node := range healthyNodes {
		contentNodes[i] = contentNode{
			DelegateOwnerWallet: node.DelegateOwnerWallet,
			Endpoint:            node.Endpoint,
		}
	}

	health := healthCheckResponse{
		Network: networkInfo{
			ContentNodes: contentNodes,
		},
	}
	return c.JSON(fiber.Map{
		"data": health,
	})
}
