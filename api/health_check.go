package api

import (
	"context"

	"api.audius.co/config"
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

	// ETL tracks the highest indexed chain height in `core_indexed_blocks`.
	// COALESCE handles the cold-start case before any blocks are indexed.
	//
	// The chain_id predicate is required for index usage. `core_indexed_blocks`
	// has a primary-key index on (chain_id, height), so the filter makes this a
	// sub-millisecond index seek. Without the filter, MAX(height) degrades to a
	// sequential scan over the whole table (~tens of millions of rows on prod)
	// at k8s probe cadence.
	//
	// We reuse the Chainid from the just-fetched nodeInfo rather than
	// plumbing a separate config field — same value ETL uses when it
	// writes rows here, so the filter always matches the writer's intent.
	var indexerLastBlockHeight int64
	err = app.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(height), 0) FROM core_indexed_blocks WHERE chain_id = $1",
		nodeInfo.Msg.Chainid,
	).Scan(&indexerLastBlockHeight)
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
