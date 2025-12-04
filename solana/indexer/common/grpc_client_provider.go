package common

import (
	"context"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// GrpcClientProvider provides GrpcClients for indexers
type GrpcClientProvider struct {
	grpcConfig      GrpcConfig
	fumaroleAdapter *FumaroleAdapter
}

// NewGrpcClientProvider creates a new provider
func NewGrpcClientProvider(grpcConfig GrpcConfig, fumaroleAdapter *FumaroleAdapter) *GrpcClientProvider {
	return &GrpcClientProvider{
		grpcConfig:      grpcConfig,
		fumaroleAdapter: fumaroleAdapter,
	}
}

// GetClient returns a GrpcClient for the given consumer group
func (p *GrpcClientProvider) GetClient(consumerGroupName string) GrpcClient {
	if p.grpcConfig.UseFumarole && p.fumaroleAdapter != nil {
		return &fumaroleClientWrapper{
			adapter:           p.fumaroleAdapter,
			consumerGroupName: consumerGroupName,
		}
	}
	return NewGrpcClient(p.grpcConfig)
}

// fumaroleClientWrapper wraps a shared Fumarole adapter to implement GrpcClient
type fumaroleClientWrapper struct {
	adapter           *FumaroleAdapter
	consumerGroupName string
}

func (w *fumaroleClientWrapper) Subscribe(
	ctx context.Context,
	subRequest *pb.SubscribeRequest,
	dataCallback DataCallback,
	errorCallback ErrorCallback,
) error {
	return w.adapter.SubscribeWithConsumerGroup(ctx, w.consumerGroupName, subRequest, dataCallback, errorCallback)
}

func (w *fumaroleClientWrapper) Close() {
	// Don't close the shared adapter
}
