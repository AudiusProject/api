package api

import (
	"context"
	"time"

	"bridgerton.audius.co/config"
	"github.com/AudiusProject/audiusd/pkg/core/contracts"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"
)

const contentNodeCacheKey = "content"
const discoveryNodeCacheKey = "discovery"

func (as *ApiServer) nodePoller(ctx context.Context) {
	as.logger.Info("starting node poller")
	err := as.refreshNodeCache()
	if err != nil {
		as.logger.Error("failed to hydrate node cache", zap.Error(err))
	}

	ticker := time.NewTicker(time.Minute * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			as.logger.Info("node poller stopping due to context cancellation")
			return
		case <-ticker.C:
			if err := as.refreshNodeCache(); err != nil {
				as.logger.Error("failed to refresh node cache", zap.Error(err))
			}
		}
	}
}

func (as *ApiServer) refreshNodeCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		discoveryNodes := []config.Node{}

		discoveryNodesResponse, err := as.ethContracts.GetAllRegisteredNodesForType(ctx, contracts.DiscoveryNode)
		if err != nil {
			return err
		}

		for _, node := range discoveryNodesResponse {
			ownerWallet := node.Owner.Hex()
			delegateOwnerWallet := node.DelegateOwnerWallet.Hex()
			discoveryNodes = append(discoveryNodes, config.Node{
				OwnerWallet:         ownerWallet,
				Endpoint:            node.Endpoint,
				IsStorageDisabled:   true,
				DelegateOwnerWallet: delegateOwnerWallet,
			})
		}

		cached := as.nodeCache.Set(discoveryNodeCacheKey, discoveryNodes)
		if !cached {
			as.logger.Error("failed to set discovery node cache")
		}
		return nil
	})

	g.Go(func() error {
		contentNodes := []config.Node{}
		contentNodesResponse, err := as.ethContracts.GetAllRegisteredNodesForType(ctx, contracts.ContentNode)
		if err != nil {
			return err
		}

		for _, node := range contentNodesResponse {
			ownerWallet := node.Owner.Hex()
			delegateOwnerWallet := node.DelegateOwnerWallet.Hex()
			contentNodes = append(contentNodes, config.Node{
				OwnerWallet:         ownerWallet,
				Endpoint:            node.Endpoint,
				IsStorageDisabled:   false,
				DelegateOwnerWallet: delegateOwnerWallet,
			})
		}

		cached := as.nodeCache.Set(contentNodeCacheKey, contentNodes)
		if !cached {
			as.logger.Error("failed to set content node cache")
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	discoveryNodes, ok := as.nodeCache.Get(discoveryNodeCacheKey)
	if !ok {
		as.logger.Error("failed to get discovery node cache")
	}

	contentNodes, ok := as.nodeCache.Get(contentNodeCacheKey)
	if !ok {
		as.logger.Error("failed to get content node cache")
	}

	as.logger.Info("refreshed node cache", zap.Int("discovery_nodes", len(discoveryNodes)), zap.Int("content_nodes", len(contentNodes)))
	return nil
}
