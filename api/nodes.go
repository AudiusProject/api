package api

import (
	"context"
	"sync"
	"time"

	"api.audius.co/config"
	"connectrpc.com/connect"
	ethv1 "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type Nodes struct {
	lock  sync.RWMutex
	nodes []config.Node
}

func NewNodes() *Nodes {
	return &Nodes{
		nodes: []config.Node{},
	}
}

// sets the nodes
func (n *Nodes) SetNodes(nodes []config.Node) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.nodes = nodes
}

// returns a copy of the nodes
func (n *Nodes) GetNodes() []config.Node {
	n.lock.RLock()
	defer n.lock.RUnlock()

	nodesCopy := make([]config.Node, len(n.nodes))
	copy(nodesCopy, n.nodes)
	return nodesCopy
}

func (app *ApiServer) nodesPoller(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	app.updateNodes(ctx)

	for {
		select {
		case <-ticker.C:
			app.updateNodes(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (app *ApiServer) updateNodes(ctx context.Context) {
	nodes, err := app.openAudioSDK.Eth.GetRegisteredEndpoints(ctx, connect.NewRequest(&ethv1.GetRegisteredEndpointsRequest{}))
	if err != nil {
		app.logger.Error("Failed to get registered nodes", zap.Error(err))
	}

	var nodesList []config.Node
	for _, node := range nodes.Msg.Endpoints {
		if node.ServiceType == "content-node" || node.ServiceType == "validator" {
			nodesList = append(nodesList, config.Node{
				Endpoint:            node.Endpoint,
				DelegateOwnerWallet: node.DelegateWallet,
				OwnerWallet:         node.Owner,
				IsStorageDisabled:   false,
			})
		}
	}

	app.validators.SetNodes(nodesList)
}

func (app *ApiServer) getValidators(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"nodes": app.validators.GetNodes(),
	})
}
