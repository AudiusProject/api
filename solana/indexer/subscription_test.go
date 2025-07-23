package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/stretchr/testify/mock"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

type mockGrpcClient struct {
	mock.Mock
}

func (m *mockGrpcClient) Subscribe(
	ctx context.Context,
	subRequest *pb.SubscribeRequest,
	dataCallback DataCallback,
	errorCallback ErrorCallback,
) error {
	args := m.Called(ctx, subRequest, dataCallback, errorCallback)
	return args.Error(0)
}

func (m *mockGrpcClient) Close() {
	m.Called()
}

type mockRpcClient struct {
	mock.Mock
}

func (m *mockRpcClient) GetBlockWithOpts(ctx context.Context, slot uint64, opts *rpc.GetBlockOpts) (*rpc.GetBlockResult, error) {
	args := m.Called(ctx, slot, opts)
	return args.Get(0).(*rpc.GetBlockResult), args.Error(1)
}

func (m *mockRpcClient) GetSlot(ctx context.Context, commitment rpc.CommitmentType) (uint64, error) {
	args := m.Called(ctx, commitment)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *mockRpcClient) GetSignaturesForAddressWithOpts(ctx context.Context, address solana.PublicKey, opts *rpc.GetSignaturesForAddressOpts) ([]*rpc.TransactionSignature, error) {
	args := m.Called(ctx, address, opts)
	return args.Get(0).([]*rpc.TransactionSignature), args.Error(1)
}

func (m *mockRpcClient) GetTransaction(ctx context.Context, signature solana.Signature, opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
	args := m.Called(ctx, signature, opts)
	return args.Get(0).(*rpc.GetTransactionResult), args.Error(1)
}

// Tests that the subscription is made for the artist coins in the database
// and is updated as new artist coins are added and removed.
func TestSubscription(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer")

	mint1 := "4k3Dyjzvzp8eXQ2f1b6d5c7g8f9h1j2k3l4m5n6o7p8q9r0s1t2u3v4w5x6y7z8"
	mint2 := "9zL1k3Dyjzvzp8eXQ2f1b6d5c7g8f9h1j2k3l4m5n6o7p8q9r0s1t2u3v4w5x6y7z8"

	database.Seed(pool, database.FixtureMap{
		"artist_coins": {
			{
				"user_id":  1,
				"mint":     mint1,
				"ticker":   "TEST",
				"decimals": 8,
			},
		},
	})

	grpcMock := &mockGrpcClient{}

	// Initial subscription should include the artist coin in the database.
	grpcMock.On("Subscribe",
		mock.Anything,
		mock.MatchedBy(func(req *pb.SubscribeRequest) bool {
			for _, account := range req.Accounts {
				for _, filter := range account.Filters {
					if f, ok := filter.Filter.(*pb.SubscribeRequestFilterAccountsFilter_Memcmp); ok {
						if f.Memcmp.GetBase58() == mint1 {
							return true
						}
					}
				}
			}
			return false
		}),
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// After inserting a new artist coin, the subscription should be updated to include it.
	grpcMock.On("Subscribe",
		mock.Anything,
		mock.MatchedBy(func(req *pb.SubscribeRequest) bool {
			foundFirst := false
			foundSecond := false
			for _, account := range req.Accounts {
				for _, filter := range account.Filters {
					if f, ok := filter.Filter.(*pb.SubscribeRequestFilterAccountsFilter_Memcmp); ok {
						if f.Memcmp.GetBase58() == mint1 {
							foundFirst = true
						}
						if f.Memcmp.GetBase58() == mint2 {
							foundSecond = true
						}
					}
				}
			}
			return foundFirst && foundSecond
		}),
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// After removing artist coins, the subscription should not include the removed mints
	grpcMock.On("Subscribe",
		mock.Anything,
		mock.MatchedBy(func(req *pb.SubscribeRequest) bool {
			for _, account := range req.Accounts {
				for _, filter := range account.Filters {
					if f, ok := filter.Filter.(*pb.SubscribeRequestFilterAccountsFilter_Memcmp); ok {
						if f.Memcmp.GetBase58() == mint1 {
							return false
						}
						if f.Memcmp.GetBase58() == mint2 {
							return false
						}
					}
				}
			}
			return true
		}),
		mock.Anything,
		mock.Anything,
	).Return(nil)

	rpcMock := &mockRpcClient{}
	rpcMock.On("GetSlot", mock.Anything, mock.Anything).
		Return(uint64(100), nil)

	s := &SolanaIndexer{
		grpcClient: grpcMock,
		rpcClient:  rpcMock,
		pool:       pool,
		logger:     zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Subscribe(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	_, err := pool.Exec(ctx, `
			INSERT INTO artist_coins (user_id, mint, ticker, decimals) 
			VALUES ($1, $2, $3, $4)
		`, 1, mint2, "TEST2", 9)
	if err != nil {
		t.Fatalf("failed to insert new artist coin: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, err = pool.Exec(ctx, "DELETE FROM artist_coins")
	if err != nil {
		t.Fatalf("failed to delete artist coins: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	cancel()

	err = <-done
	assert.True(t, errors.Is(err, context.Canceled), err.Error())
	grpcMock.AssertExpectations(t)
}
