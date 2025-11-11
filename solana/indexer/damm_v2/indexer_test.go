package damm_v2

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/indexer/fake_rpc_client"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/stretchr/testify/mock"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestHandleUpdate_SlotCheckpoint(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	expectedSlot := uint64(1500)

	request := pb.SubscribeRequest{}
	checkpointId, err := common.InsertCheckpointStart(t.Context(), pool, "test", 1000, &request)
	update := pb.SubscribeUpdate{
		Filters: []string{checkpointId},
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot: expectedSlot,
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	slot, err := common.GetCheckpointSlot(t.Context(), pool, "test", &request)
	require.NoError(t, err)
	assert.Equal(t, expectedSlot, slot, "checkpoint slot should be updated")
}

func TestHandleUpdate_DammV2PoolUpdate(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	// From real on-chain account data
	address := solana.MustPublicKeyFromBase58("D9iJqMbgQJLFt5PAAiTJTMNsMAMueukzoe1EK2r1g3WH")
	poolBase64 := "8ZptBBGxbbyAlpgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFAAUAAAAAAABAAAAAAAAAGCk3AC8AwAAAQAKAHgAiBPGROhoAAAAAMsQx7q4jQYAAAAAAAAAAAChIqYBNRzVAQAAAAAAAAAA4CICAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACOkzYTTyijBQphnsA7NYukXXDff56Bp/GJdn5GamlMZ7/DPMLnXBSHbMN5KDkE9JB3ZpESJXuzrf82mLYCJJQHm/3HSrkp1wPzAbe6y0uFypnr4Yeci2kPU8TWr9TEmTY/2aZknQDPJED5N2M3ytBL5gl4lD8TdKznaJkMDHT44AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAANpjaB9yhrzIBnGeLCtQogFXJDv8lmixFSC8U4Q+3NsISFBg5M0NQHAAaMJHGBEGAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFiinY8UAAAAAAAAAAAAAAAAAAAAAAAAAFA7AQABAAAAAAAAAAAAAACbV2lOqRpchLHE/v8AAAAAIiTN1Ql11QEAAAAAAAAAAIhx5mgAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAJE91kBWow0AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAASFBg5M0NQHAAaMJHGBEGAAAAAAAAAAAAAAAAAAAAAAASYim9UgAAAAAAAAAAAAAAAAAAAAAAAABYop2PFAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAA2mNoH3KGvMgGcZ4sK1CiAVckO/yWaLEVILxThD7c2wgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	poolData, err := base64.StdEncoding.DecodeString(poolBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		Filters: []string{NAME},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Slot: 123456789,
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: address.Bytes(),
					Data:   poolData,
				},
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	// Verify the DAMM v2 pool was inserted
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_damm_v2_pools
			WHERE account = @account
				AND slot = @slot
				AND token_a_mint = @token_a_mint
				AND token_b_mint = @token_b_mint
				AND token_a_vault = @token_a_vault
				AND token_b_vault = @token_b_vault
				AND whitelisted_vault = @whitelisted_vault
				AND partner = @partner
				AND liquidity = @liquidity
				AND protocol_a_fee = @protocol_a_fee
				AND protocol_b_fee = @protocol_b_fee
				AND partner_a_fee = @partner_a_fee
				AND partner_b_fee = @partner_b_fee
				AND sqrt_min_price = @sqrt_min_price
				AND sqrt_max_price = @sqrt_max_price
				AND sqrt_price = @sqrt_price
				AND activation_point = @activation_point
				AND activation_type = @activation_type
				AND pool_status = @pool_status
				AND token_a_flag = @token_a_flag
				AND token_b_flag = @token_b_flag
				AND collect_fee_mode = @collect_fee_mode
				AND pool_type = @pool_type
				AND version = @version
				AND fee_a_per_liquidity = @fee_a_per_liquidity
				AND fee_b_per_liquidity = @fee_b_per_liquidity
				AND permanent_lock_liquidity = @permanent_lock_liquidity
				AND creator = @creator
			LIMIT 1
		)
	`

	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"account":                  address.String(),
		"slot":                     int64(123456789),
		"token_a_mint":             "bnWKPK7YTUJTe3A3HTGEJrUEoAddRgRjWSwf7MwxMP3",
		"token_b_mint":             "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"token_a_vault":            "9CG1qU4bhiGX9J5k5Ap1hkWpWftV5fjWJi3h8FMUvaGJ",
		"token_b_vault":            "7jKehVD6cxYtNmcqLqyFb3W4xtaWjfTf7CWgbWLkQ7ru",
		"whitelisted_vault":        "11111111111111111111111111111111",
		"partner":                  "FhVo3mqL8PW5pH5U2CN4XE33DokiyZnUwuGpH2hmHLuM",
		"liquidity":                "31500505798829827035928817465053256",
		"protocol_a_fee":           uint64(0),
		"protocol_b_fee":           uint64(88308818520),
		"partner_a_fee":            uint64(0),
		"partner_b_fee":            uint64(0),
		"sqrt_min_price":           uint64(4295048016),
		"sqrt_max_price":           "79226673521066979257578248091",
		"sqrt_price":               "132140449179444258",
		"activation_point":         int64(1759932808),
		"activation_type":          uint8(1),
		"pool_status":              uint8(0),
		"token_a_flag":             uint8(0),
		"token_b_flag":             uint8(0),
		"collect_fee_mode":         uint8(1),
		"pool_type":                uint8(0),
		"version":                  uint8(0),
		"fee_a_per_liquidity":      uint64(0),
		"fee_b_per_liquidity":      "3838765547535761",
		"permanent_lock_liquidity": "31500505798829827035928817465053256",
		"creator":                  "FhVo3mqL8PW5pH5U2CN4XE33DokiyZnUwuGpH2hmHLuM",
	}).Scan(&exists)
	require.NoError(t, err, "failed to query for damm v2 pool")
	assert.True(t, exists, "damm v2 pool should exist after indexing")

	rows, err := pool.Query(t.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_damm_v2_pools
			JOIN sol_meteora_damm_v2_pool_metrics ON sol_meteora_damm_v2_pool_metrics.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_fees ON sol_meteora_damm_v2_pool_fees.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_base_fees ON sol_meteora_damm_v2_pool_base_fees.pool = sol_meteora_damm_v2_pools.account
			JOIN sol_meteora_damm_v2_pool_dynamic_fees ON sol_meteora_damm_v2_pool_dynamic_fees.pool = sol_meteora_damm_v2_pools.account
			LIMIT 1
		)
	`)
	require.NoError(t, err)
	defer rows.Close()
}

func TestHandleUpdate_DammV2PositionUpdate(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")
	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)

	// From real on-chain account data
	address := solana.MustPublicKeyFromBase58("5bYLydDXt1K5zroychcbrVbhGRUpheXdq5w41uccazPB")
	positionBase64 := "qryP5HpA99C0h5iaMb9or5qzYmaPKH7cBpP1GTyw5pa9SMlEQMuk4oeLsnqCTyioPLOFt664lEHr2woSYFq4Z3N6xFLWwGDSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADUszHGm5oNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACQoMPLmBiA4ADThI4wIAwAAAAAAAAAAABGmGkQpAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	positionData, err := base64.StdEncoding.DecodeString(positionBase64)
	require.NoError(t, err)

	update := pb.SubscribeUpdate{
		Filters: []string{address.String()},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: address.Bytes(),
					Data:   positionData,
				},
			},
		},
	}

	err = indexer.HandleUpdate(t.Context(), &update)
	require.NoError(t, err)

	rows, err := pool.Query(t.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM sol_meteora_damm_v2_positions
			JOIN sol_meteora_damm_v2_position_metrics ON sol_meteora_damm_v2_position_metrics.position = sol_meteora_damm_v2_positions.account
			LIMIT 1
		)
	`)
	require.NoError(t, err)
	defer rows.Close()
}

type grpcClientMock struct {
	mock.Mock

	onUpdate common.DataCallback
}

func (m *grpcClientMock) Subscribe(ctx context.Context, req *pb.SubscribeRequest, onUpdate common.DataCallback, onError common.ErrorCallback) error {
	args := m.Called(ctx, req, onUpdate, onError)
	m.onUpdate = onUpdate
	return args.Error(0)
}

func (m *grpcClientMock) Close() {
	m.Called()
}

func TestSubscription(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_solana_indexer_damm_v2")

	// Fake an update for a Position and a Pool with missing data (should fail)
	positionAddress := solana.MustPublicKeyFromBase58("5bYLydDXt1K5zroychcbrVbhGRUpheXdq5w41uccazPB")
	positionUpdate := pb.SubscribeUpdate{
		Filters: []string{positionAddress.String()},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: positionAddress.Bytes(),
					Data:   append(meteora_damm_v2.POSITION_DISCRIMINATOR[:], 0), // incomplete data to trigger failure
				},
			},
		},
	}
	dammPoolAddress := solana.MustPublicKeyFromBase58("D9iJqMbgQJLFt5PAAiTJTMNsMAMueukzoe1EK2r1g3WH")
	poolUpdate := pb.SubscribeUpdate{
		Filters: []string{NAME},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: dammPoolAddress.Bytes(),
					Data:   append(meteora_damm_v2.POOL_DISCRIMINATOR[:], 0), // incomplete data to trigger failure
				},
			},
		},
	}
	dammPoolAddress2 := solana.MustPublicKeyFromBase58("8Z8rCYLuUcLfAJYPZxMgn6i9ifg9znQxrckXgZh6kYvN")

	database.Seed(pool, database.FixtureMap{
		"artist_coins": []map[string]any{
			{
				"mint":         "abc",
				"ticker":       "",
				"user_id":      0,
				"decimals":     9,
				"damm_v2_pool": dammPoolAddress.String(),
			},
		},
	})

	rpcClient := fake_rpc_client.FakeRpcClient{}
	logger := zap.NewNop()

	grpcMock := grpcClientMock{}
	grpcMock.On("Subscribe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	grpcMock.On("Close").Return()

	indexer := New(common.GrpcConfig{}, &rpcClient, pool, logger)
	indexer.grpcFactory = func(config common.GrpcConfig) common.GrpcClient {
		return &grpcMock
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()
	go indexer.Start(ctx)

	for grpcMock.onUpdate == nil {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for subscription to be created")
		default:
		}
	}

	// Assert the original subscription included the actual account
	grpcMock.AssertCalled(t, "Subscribe", mock.Anything, mock.MatchedBy(func(req *pb.SubscribeRequest) bool {
		hasDammFilter := len(req.Accounts[NAME].Account) == 1 &&
			req.Accounts[NAME].Account[0] == dammPoolAddress.String()
		hasPositionFilter := req.Accounts[dammPoolAddress.String()].
			Filters[1].
			Filter.(*pb.SubscribeRequestFilterAccountsFilter_Memcmp).
			Memcmp.
			Data.(*pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58).
			Base58 == dammPoolAddress.String()
		return hasDammFilter && hasPositionFilter
	}), mock.Anything, mock.Anything)

	// Send the updates (with missing data)
	grpcMock.onUpdate(ctx, &positionUpdate)
	grpcMock.onUpdate(ctx, &poolUpdate)
	grpcMock.onUpdate = nil

	// Update the DB to trigger a refresh of the subscription
	sql := `UPDATE artist_coins SET damm_v2_pool = @damm_v2_pool WHERE mint = 'abc'`
	_, err := pool.Exec(ctx, sql, pgx.NamedArgs{
		"damm_v2_pool": dammPoolAddress2.String(),
	})
	require.NoError(t, err)

	for grpcMock.onUpdate == nil {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for subscription to be created")
		default:
		}
	}

	cancel()

	// Assert that on refresh, the subscription included the updated account
	grpcMock.AssertCalled(t, "Subscribe", mock.Anything, mock.MatchedBy(func(req *pb.SubscribeRequest) bool {
		hasDammFilter := len(req.Accounts[NAME].Account) == 1 &&
			req.Accounts[NAME].Account[0] == dammPoolAddress2.String()
		hasPositionFilter := req.Accounts[dammPoolAddress2.String()].
			Filters[1].
			Filter.(*pb.SubscribeRequestFilterAccountsFilter_Memcmp).
			Memcmp.
			Data.(*pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58).
			Base58 == dammPoolAddress2.String()
		return hasDammFilter && hasPositionFilter
	}), mock.Anything, mock.Anything)

	// Assert that the updates with missing data were added to the retry queue
	positionUpdateJson, err := protojson.Marshal(common.RetryQueueUpdate{SubscribeUpdate: &positionUpdate})
	require.NoError(t, err)
	poolUpdateJson, err := protojson.Marshal(common.RetryQueueUpdate{SubscribeUpdate: &poolUpdate})
	require.NoError(t, err)

	var exists bool
	sql = `SELECT EXISTS (SELECT 1 FROM sol_retry_queue WHERE update_message = @update_message)`
	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"update_message": positionUpdateJson,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "failed position update should be added to retry queue")

	err = pool.QueryRow(t.Context(), sql, pgx.NamedArgs{
		"update_message": poolUpdateJson,
	}).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "failed pool update should be added to retry queue")
}
