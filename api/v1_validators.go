package api

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
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
	if nodes == nil || nodes.Msg == nil {
		app.logger.Error("GetRegisteredEndpoints returned nil response")
		return
	}

	var nodesList []config.Node
	for _, node := range nodes.Msg.Endpoints {
		nodesList = append(nodesList, config.Node{
			Id:             strconv.FormatInt(node.Id, 10),
			Endpoint:       node.Endpoint,
			DelegateWallet: node.DelegateWallet,
			Owner:          node.Owner,
			ServiceType:    node.ServiceType,
			RegisteredAt:   node.RegisteredAt.AsTime().Format(time.RFC3339),
			BlockNumber:    node.BlockNumber,
		})
	}
	app.validators.SetNodes(nodesList)
	rendezvousNodes := make([]config.Node, 0, len(nodesList))
	for _, n := range nodesList {
		if strings.EqualFold(n.ServiceType, "validator") || strings.EqualFold(n.ServiceType, "content-node") {
			rendezvousNodes = append(rendezvousNodes, n)
		}
	}
	rendezvous.Refresh(rendezvousNodes)
}

func (app *ApiServer) v1Validators(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"nodes": app.validators.GetNodes(),
	})
}
