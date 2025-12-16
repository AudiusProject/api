package api

import (
	"context"

	"api.audius.co/config"
	core_indexer "api.audius.co/indexer"
	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type contentNode struct {
	// camelCase to maintain backwards compatibility
	DelegateOwnerWallet string `json:"delegateOwnerWallet"`
	Endpoint            string `json:"endpoint"`
}

type networkInfo struct {
	Validators   []config.Node `json:"validators"`
	ContentNodes []contentNode `json:"content_nodes"`
}

type healthCheckResponse struct {
	CoreIndexer *coreIndexerHealth `json:"core_indexer"`
	Network     networkInfo        `json:"network"`
}

type coreIndexerHealth struct {
	LastIndexedBlock int64 `json:"last_indexed_block"`
	BlockDiff        int64 `json:"block_diff"`
}

func (app *ApiServer) getCoreIndexerHealth(ctx context.Context) (*coreIndexerHealth, error) {
	nodeInfo, err := app.openAudioSDK.Core.GetNodeInfo(ctx, connect.NewRequest(&corev1.GetNodeInfoRequest{}))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get core node info")
	}
	chainHeight := nodeInfo.Msg.CurrentHeight

	var indexerLastBlockHeight int64
	err = app.pool.QueryRow(ctx, "SELECT COALESCE(last_checkpoint, 0) FROM indexing_checkpoints WHERE tablename = $1", core_indexer.CoreIndexerCheckpointName).Scan(&indexerLastBlockHeight)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get core indexer last block height")
	}

	blockDiff := chainHeight - indexerLastBlockHeight

	return &coreIndexerHealth{
		LastIndexedBlock: indexerLastBlockHeight,
		BlockDiff:        blockDiff,
	}, nil
}

type HealthCheckQueryParams struct {
	MaxCoreIndexerBlockDiff *int64 `query:"max_core_indexer_block_diff" validate:"omitempty,min=0"`
}

func (app *ApiServer) healthCheck(c *fiber.Ctx) error {
	var params HealthCheckQueryParams
	err := app.ParseAndValidateQueryParams(c, &params)
	if err != nil {
		return err
	}

	coreIndexerHealth, err := app.getCoreIndexerHealth(c.Context())
	if err != nil {
		app.logger.Error("Failed to get core indexer health", zap.Error(err))
	}

	if params.MaxCoreIndexerBlockDiff != nil {
		// If max diff was requested but we failed to calculate it,
		// return 500 just to be safe.
		if coreIndexerHealth == nil || coreIndexerHealth.BlockDiff > *params.MaxCoreIndexerBlockDiff {
			c.Status(fiber.StatusInternalServerError)
		}
	}

	nodes := app.validators.GetNodes()
	contentNodes := make([]contentNode, 0)
	for _, node := range nodes {
		for _, uploadNode := range app.config.UploadNodes {
			if node.Endpoint == uploadNode {
				contentNodes = append(contentNodes, contentNode{
					DelegateOwnerWallet: node.DelegateWallet,
					Endpoint:            node.Endpoint,
				})
				break
			}
		}
	}

	health := healthCheckResponse{
		CoreIndexer: coreIndexerHealth,
		Network: networkInfo{
			Validators:   nodes,
			ContentNodes: contentNodes,
		},
	}
	return c.JSON(fiber.Map{
		"data": health,
	})
}
