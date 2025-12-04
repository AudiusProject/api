package common

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

type RpcClient interface {
	GetBlockWithOpts(context.Context, uint64, *rpc.GetBlockOpts) (*rpc.GetBlockResult, error)
	GetSlot(context.Context, rpc.CommitmentType) (uint64, error)
	GetSignaturesForAddressWithOpts(context.Context, solana.PublicKey, *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error)
	GetTransaction(context.Context, solana.Signature, *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)
	GetAccountDataBorshInto(ctx context.Context, account solana.PublicKey, out interface{}) error
}

type GrpcClient interface {
	Subscribe(
		ctx context.Context,
		subRequest *pb.SubscribeRequest,
		dataCallback DataCallback,
		errorCallback ErrorCallback,
	) error
	Close()
}

type GrpcConfig struct {
	Server                string
	ApiToken              string
	MaxReconnectAttempts  int
	UseFumarole           bool
	FumaroleConsumerGroup string
	DebugLogging          bool
}
