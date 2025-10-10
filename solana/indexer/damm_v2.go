package indexer

import (
	"context"
	"fmt"

	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type notificationCallback func(ctx context.Context, notification *pgconn.Notification)

type DammV2Indexer struct {
	pool       database.DbPool
	grpcConfig GrpcConfig
	logger     *zap.Logger
}

const MAX_DAMM_V2_POOLS_PER_SUBSCRIPTION = 10000
const DAMM_V2_POOL_SUBSCRIPTION_KEY = "dammV2Pools"
const DBC__MIGRATION_NOTIFICATION_NAME = "meteora_dbc_migration"

func (d *DammV2Indexer) Start(ctx context.Context) {
	// To ensure only one subscription task is running at a time, keep track of
	// the last cancel function and call it on the next notification.
	var lastCancel context.CancelFunc

	// Ensure all gRPC clients are closed on shutdown
	var grpcClients []GrpcClient
	defer (func() {
		for _, client := range grpcClients {
			client.Close()
		}
	})()

	handleNotif := func(ctx context.Context, notification *pgconn.Notification) {
		// Cancel the previous task if it exists
		subCtx, cancel := context.WithCancel(ctx)
		if lastCancel != nil {
			lastCancel()
		}
		for _, client := range grpcClients {
			client.Close()
		}
		clients, err := subscribeToDammV2Pools(subCtx, d.pool, d.grpcConfig, d.logger)
		grpcClients = clients
		if err != nil {
			d.logger.Error("failed to resubscribe to DAMM V2 pools", zap.Error(err))
			return
		}
		lastCancel = cancel
	}

	// Setup initial subscription
	clients, err := subscribeToDammV2Pools(ctx, d.pool, d.grpcConfig, d.logger)
	if err != nil {
		d.logger.Error("failed to subscribe to DAMM V2 pools", zap.Error(err))
		return
	}
	grpcClients = clients

	// Watch for new pools to be added
	err = watchPgNotification(ctx, d.pool, DBC__MIGRATION_NOTIFICATION_NAME, handleNotif, d.logger)
	if err != nil {
		d.logger.Error("failed to watch for DAMM V2 pool changes", zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("received shutdown signal, stopping DAMM V2 indexer")
			return
		default:
		}
	}
}

func subscribeToDammV2Pools(ctx context.Context, db database.DBTX, grpcConfig GrpcConfig, logger *zap.Logger) ([]GrpcClient, error) {
	done := false
	page := 0
	pageSize := MAX_DAMM_V2_POOLS_PER_SUBSCRIPTION
	total := 0
	grpcClients := make([]GrpcClient, 0)
	for !done {
		dammV2Pools, err := getWatchedDammV2Pools(ctx, db, pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get watched DAMM V2 pools: %w", err)
		}
		if len(dammV2Pools) == 0 {
			logger.Info("no DAMM V2 pools to subscribe to")
			return grpcClients, nil
		}
		total += len(dammV2Pools)

		logger.Debug("subscribing to DAMM V2 pools....", zap.Int("numPools", len(dammV2Pools)))
		subscription := makeDammV2SubscriptionRequest(dammV2Pools)

		handleMessage := func(ctx context.Context, msg *pb.SubscribeUpdate) {
			handleDammV2Message(ctx, db, msg, logger)
		}

		grpcClient := NewGrpcClient(grpcConfig)
		err = grpcClient.Subscribe(ctx, subscription, handleMessage, func(err error) {
			logger.Error("error in DAMM V2 subscription", zap.Error(err))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to DAMM V2 pools: %w", err)
		}
		grpcClients = append(grpcClients, grpcClient)

		if len(dammV2Pools) < pageSize {
			done = true
		}
		page++
	}
	logger.Info("subscribed to DAMM V2 pools", zap.Int("numPools", total))
	return grpcClients, nil
}

func watchPgNotification(ctx context.Context, pool database.DbPool, notification string, callback notificationCallback, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}

	childLogger := logger.With(zap.String("notification", notification))

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection: %w", err)
	}

	rawConn := conn.Conn()
	_, err = rawConn.Exec(ctx, fmt.Sprintf(`LISTEN %s`, notification))
	if err != nil {
		return fmt.Errorf("failed to listen for %s changes: %w", notification, err)
	}

	go func() {
		defer func() {
			if rawConn != nil && !rawConn.PgConn().IsClosed() && ctx.Err() != nil {
				_, _ = rawConn.Exec(ctx, fmt.Sprintf(`UNLISTEN %s`, notification))
			}
			childLogger.Info("received shutdown signal, stopping notification watcher")
			conn.Release()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			notif, err := rawConn.WaitForNotification(ctx)
			if err != nil {
				childLogger.Error("failed waiting for notification", zap.Error(err))
			}
			if notif == nil {
				childLogger.Warn("received nil notification, continuing to wait for notifications")
				continue
			}
			callback(ctx, notif)
		}
	}()
	return nil
}

func makeDammV2SubscriptionRequest(dammV2Pools []string) *pb.SubscribeRequest {
	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
	}

	// Listen for slot updates for checkpointing
	subscription.Slots = make(map[string]*pb.SubscribeRequestFilterSlots)
	subscription.Slots["checkpoints"] = &pb.SubscribeRequestFilterSlots{}

	// fromSlot := uint64(372380625)
	// subscription.FromSlot = &fromSlot

	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)

	// Listen to all watched pools
	accountFilter := pb.SubscribeRequestFilterAccounts{
		Owner:   []string{meteora_damm_v2.ProgramID.String()},
		Account: dammV2Pools,
	}
	subscription.Accounts[DAMM_V2_POOL_SUBSCRIPTION_KEY] = &accountFilter

	// Listen to all positions for each pool
	for _, pool := range dammV2Pools {
		accountFilter := pb.SubscribeRequestFilterAccounts{
			Owner: []string{meteora_damm_v2.ProgramID.String()},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: 8, // Offset of the pool field in the position account (after discriminator)
							Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
								Base58: pool,
							},
						},
					},
				},
				{
					Filter: &pb.SubscribeRequestFilterAccountsFilter_Datasize{
						Datasize: 408, // byte size of a Position account
					},
				},
			},
		}
		subscription.Accounts[pool] = &accountFilter
	}

	return subscription
}

func handleDammV2Message(ctx context.Context, db database.DBTX, msg *pb.SubscribeUpdate, logger *zap.Logger) {
	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		if msg.Filters[0] == DAMM_V2_POOL_SUBSCRIPTION_KEY {
			err := processDammV2PoolUpdate(ctx, db, accUpdate)
			if err != nil {
				logger.Error("failed to process DAMM V2 pool update", zap.Error(err))
			} else {
				logger.Debug("processed DAMM V2 pool update", zap.String("account", solana.PublicKeyFromBytes(accUpdate.Account.Pubkey).String()))
			}
		} else {
			err := processDammV2PositionUpdate(ctx, db, accUpdate)
			if err != nil {
				logger.Error("failed to process DAMM V2 position update", zap.Error(err))
			} else {
				logger.Debug("processed DAMM V2 position update", zap.String("account", solana.PublicKeyFromBytes(accUpdate.Account.Pubkey).String()))
			}
		}

	}
}

func processDammV2PoolUpdate(
	ctx context.Context,
	db database.DBTX,
	update *pb.SubscribeUpdateAccount,
) error {
	account := solana.PublicKeyFromBytes(update.Account.Pubkey)
	var pool meteora_damm_v2.Pool
	err := bin.NewBorshDecoder(update.Account.Data).Decode(&pool)
	if err != nil {
		return err
	}
	err = upsertDammV2Pool(ctx, db, account, &pool)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolMetrics(ctx, db, account, &pool.Metrics)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolFees(ctx, db, account, &pool.PoolFees)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolBaseFee(ctx, db, account, &pool.PoolFees.BaseFee)
	if err != nil {
		return err
	}
	err = upsertDammV2PoolDynamicFee(ctx, db, account, &pool.PoolFees.DynamicFee)
	if err != nil {
		return err
	}
	return nil
}

func processDammV2PositionUpdate(
	ctx context.Context,
	db database.DBTX,
	update *pb.SubscribeUpdateAccount,
) error {
	account := solana.PublicKeyFromBytes(update.Account.Pubkey)
	var position meteora_damm_v2.PositionState
	err := bin.NewBorshDecoder(update.Account.Data).Decode(&position)
	if err != nil {
		return err
	}
	err = upsertDammV2Position(ctx, db, account, &position)
	if err != nil {
		return err
	}
	err = upsertDammV2PositionMetrics(ctx, db, account, &position.Metrics)
	if err != nil {
		return err
	}
	return nil
}

func getWatchedDammV2Pools(ctx context.Context, db database.DBTX, limit int, offset int) ([]string, error) {
	sql := `
		SELECT damm_v2_pool
		FROM sol_meteora_dbc_migrations
		LIMIT @limit OFFSET @offset
	;`
	rows, err := db.Query(ctx, sql, pgx.NamedArgs{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []string
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, err
		}
		pools = append(pools, address)
	}
	return pools, nil
}

func upsertDammV2Pool(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	pool *meteora_damm_v2.Pool,
) error {
	sqlPool := `
		INSERT INTO sol_meteora_damm_v2_pools (
			address,
			token_a_mint,
			token_b_mint,
			token_a_vault,
			token_b_vault,
			whitelisted_vault,
			partner,
			liquidity,
			protocol_a_fee,
			protocol_b_fee,
			partner_a_fee,
			partner_b_fee,
			sqrt_min_price,
			sqrt_max_price,
			sqrt_price,
			activation_point,
			activation_type,
			pool_status,
			token_a_flag,
			token_b_flag,
			collect_fee_mode,
			pool_type,
			fee_a_per_liquidity,
			fee_b_per_liquidity,
			permanent_lock_liquidity,
			creator,
			created_at,
			updated_at
		) VALUES (
			@address,
			@token_a_mint,
			@token_b_mint,
			@token_a_vault,
			@token_b_vault,
			@whitelisted_vault,
			@partner,
			@liquidity,
			@protocol_a_fee,
			@protocol_b_fee,
			@partner_a_fee,
			@partner_b_fee,
			@sqrt_min_price,
			@sqrt_max_price,
			@sqrt_price,
			@activation_point,
			@activation_type,
			@pool_status,
			@token_a_flag,
			@token_b_flag,
			@collect_fee_mode,
			@pool_type,
			@fee_a_per_liquidity,
			@fee_b_per_liquidity,
			@permanent_lock_liquidity,
			@creator,
			NOW(),
			NOW()
		)
		ON CONFLICT (address) DO UPDATE SET
			token_a_mint = EXCLUDED.token_a_mint,
			token_b_mint = EXCLUDED.token_b_mint,
			token_a_vault = EXCLUDED.token_a_vault,
			token_b_vault = EXCLUDED.token_b_vault,
			whitelisted_vault = EXCLUDED.whitelisted_vault,
			partner = EXCLUDED.partner,
			liquidity = EXCLUDED.liquidity,
			protocol_a_fee = EXCLUDED.protocol_a_fee,
			protocol_b_fee = EXCLUDED.protocol_b_fee,
			partner_a_fee = EXCLUDED.partner_a_fee,
			partner_b_fee = EXCLUDED.partner_b_fee,
			sqrt_min_price = EXCLUDED.sqrt_min_price,
			sqrt_max_price = EXCLUDED.sqrt_max_price,
			sqrt_price = EXCLUDED.sqrt_price,
			activation_point = EXCLUDED.activation_point,
			activation_type = EXCLUDED.activation_type,
			pool_status = EXCLUDED.pool_status,
			token_a_flag = EXCLUDED.token_a_flag,
			token_b_flag = EXCLUDED.token_b_flag,
			collect_fee_mode = EXCLUDED.collect_fee_mode,
			pool_type = EXCLUDED.pool_type,
			fee_a_per_liquidity = EXCLUDED.fee_a_per_liquidity,
			fee_b_per_liquidity = EXCLUDED.fee_b_per_liquidity,
			permanent_lock_liquidity = EXCLUDED.permanent_lock_liquidity,
			creator = EXCLUDED.creator,
			updated_at = NOW()
	`
	args := pgx.NamedArgs{
		"address":                  account.String(),
		"token_a_mint":             pool.TokenAMint.String(),
		"token_b_mint":             pool.TokenBMint.String(),
		"token_a_vault":            pool.TokenAVault.String(),
		"token_b_vault":            pool.TokenBVault.String(),
		"whitelisted_vault":        pool.WhitelistedVault.String(),
		"partner":                  pool.Partner.String(),
		"liquidity":                pool.Liquidity.String(),
		"protocol_a_fee":           pool.Metrics.TotalProtocolAFee,
		"protocol_b_fee":           pool.Metrics.TotalProtocolBFee,
		"partner_a_fee":            pool.Metrics.TotalPartnerAFee,
		"partner_b_fee":            pool.Metrics.TotalPartnerBFee,
		"sqrt_min_price":           pool.SqrtMinPrice.BigInt(),
		"sqrt_max_price":           pool.SqrtMaxPrice.BigInt(),
		"sqrt_price":               pool.SqrtPrice.BigInt(),
		"activation_point":         pool.ActivationPoint,
		"activation_type":          pool.ActivationType,
		"pool_status":              pool.PoolStatus,
		"token_a_flag":             pool.TokenAFlag,
		"token_b_flag":             pool.TokenBFlag,
		"collect_fee_mode":         pool.CollectFeeMode,
		"pool_type":                pool.PoolType,
		"fee_a_per_liquidity":      pool.FeeAPerLiquidity,
		"fee_b_per_liquidity":      pool.FeeBPerLiquidity,
		"permanent_lock_liquidity": pool.PermanentLockLiquidity.BigInt(),
		"creator":                  pool.Creator.String(),
	}
	_, err := db.Exec(ctx, sqlPool, args)

	return err
}

func upsertDammV2PoolMetrics(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	metrics *meteora_damm_v2.PoolMetrics,
) error {
	sqlMetrics := `
		INSERT INTO sol_meteora_damm_v2_pool_metrics (
			pool,
			total_lp_a_fee,
			total_lp_b_fee,
			total_protocol_a_fee,
			total_protocol_b_fee,
			total_partner_a_fee,
			total_partner_b_fee,
			total_position,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@total_lp_a_fee,
			@total_lp_b_fee,
			@total_protocol_a_fee,
			@total_protocol_b_fee,
			@total_partner_a_fee,
			@total_partner_b_fee,
			@total_position,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			total_lp_a_fee = EXCLUDED.total_lp_a_fee,
			total_lp_b_fee = EXCLUDED.total_lp_b_fee,
			total_protocol_a_fee = EXCLUDED.total_protocol_a_fee,
			total_protocol_b_fee = EXCLUDED.total_protocol_b_fee,
			total_partner_a_fee = EXCLUDED.total_partner_a_fee,
			total_partner_b_fee = EXCLUDED.total_partner_b_fee,
			total_position = EXCLUDED.total_position,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sqlMetrics, pgx.NamedArgs{
		"pool":                 account.String(),
		"total_lp_a_fee":       metrics.TotalLpAFee,
		"total_lp_b_fee":       metrics.TotalLpBFee,
		"total_protocol_a_fee": metrics.TotalProtocolAFee,
		"total_protocol_b_fee": metrics.TotalProtocolBFee,
		"total_partner_a_fee":  metrics.TotalPartnerAFee,
		"total_partner_b_fee":  metrics.TotalPartnerBFee,
		"total_position":       metrics.TotalPosition,
	})
	return err
}

func upsertDammV2PoolFees(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	fees *meteora_damm_v2.PoolFeesStruct,
) error {
	sqlFees := `
		INSERT INTO sol_meteora_damm_v2_pool_fees (
			pool,
			partner_fee_percent,
			protocol_fee_percent,
			referral_fee_percent,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@partner_fee_percent,
			@protocol_fee_percent,
			@referral_fee_percent,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			partner_fee_percent = EXCLUDED.partner_fee_percent,
			protocol_fee_percent = EXCLUDED.protocol_fee_percent,
			referral_fee_percent = EXCLUDED.referral_fee_percent,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sqlFees, pgx.NamedArgs{
		"pool":                 account.String(),
		"partner_fee_percent":  fees.PartnerFeePercent,
		"protocol_fee_percent": fees.ProtocolFeePercent,
		"referral_fee_percent": fees.ReferralFeePercent,
	})
	return err
}

func upsertDammV2PoolBaseFee(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	baseFee *meteora_damm_v2.BaseFeeStruct,
) error {
	sqlBaseFee := `
		INSERT INTO sol_meteora_damm_v2_pool_base_fees (
			pool,
			cliff_fee_numerator,
			fee_scheduler_mode,
			number_of_period,
			period_frequency,
			reduction_factor,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@cliff_fee_numerator,
			@fee_scheduler_mode,
			@number_of_period,
			@period_frequency,
			@reduction_factor,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			cliff_fee_numerator = EXCLUDED.cliff_fee_numerator,
			fee_scheduler_mode = EXCLUDED.fee_scheduler_mode,
			number_of_period = EXCLUDED.number_of_period,
			period_frequency = EXCLUDED.period_frequency,
			reduction_factor = EXCLUDED.reduction_factor,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sqlBaseFee, pgx.NamedArgs{
		"pool":                account.String(),
		"cliff_fee_numerator": baseFee.CliffFeeNumerator,
		"fee_scheduler_mode":  baseFee.FeeSchedulerMode,
		"number_of_period":    baseFee.NumberOfPeriod,
		"period_frequency":    baseFee.PeriodFrequency,
		"reduction_factor":    baseFee.ReductionFactor,
	})
	return err
}

func upsertDammV2PoolDynamicFee(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	dynamicFee *meteora_damm_v2.DynamicFeeStruct,
) error {
	sqlDynamicFee := `
		INSERT INTO sol_meteora_damm_v2_pool_dynamic_fees (
			pool,
			initialized,
			max_volatility_accumulator,
			variable_fee_control,
			bin_step,
			filter_period,
			decay_period,
			reduction_factor,
			last_update_timestamp,
			bin_step_u128,
			sqrt_price_reference,
			volatility_accumulator,
			volatility_reference,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@initialized,
			@max_volatility_accumulator,
			@variable_fee_control,
			@bin_step,
			@filter_period,
			@decay_period,
			@reduction_factor,
			@last_update_timestamp,
			@bin_step_u128,
			@sqrt_price_reference,
			@volatility_accumulator,
			@volatility_reference,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			initialized = EXCLUDED.initialized,
			max_volatility_accumulator = EXCLUDED.max_volatility_accumulator,
			variable_fee_control = EXCLUDED.variable_fee_control,
			bin_step = EXCLUDED.bin_step,
			filter_period = EXCLUDED.filter_period,
			decay_period = EXCLUDED.decay_period,
			reduction_factor = EXCLUDED.reduction_factor,
			last_update_timestamp = EXCLUDED.last_update_timestamp,
			bin_step_u128 = EXCLUDED.bin_step_u128,
			sqrt_price_reference = EXCLUDED.sqrt_price_reference,
			volatility_accumulator = EXCLUDED.volatility_accumulator,
			volatility_reference = EXCLUDED.volatility_reference,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sqlDynamicFee, pgx.NamedArgs{
		"pool":                       account.String(),
		"initialized":                dynamicFee.Initialized,
		"max_volatility_accumulator": dynamicFee.MaxVolatilityAccumulator,
		"variable_fee_control":       dynamicFee.VariableFeeControl,
		"bin_step":                   dynamicFee.BinStep,
		"filter_period":              dynamicFee.FilterPeriod,
		"decay_period":               dynamicFee.DecayPeriod,
		"reduction_factor":           dynamicFee.ReductionFactor,
		"last_update_timestamp":      dynamicFee.LastUpdateTimestamp,
		"bin_step_u128":              dynamicFee.BinStepU128,
		"sqrt_price_reference":       dynamicFee.SqrtPriceReference,
		"volatility_accumulator":     dynamicFee.VolatilityAccumulator,
		"volatility_reference":       dynamicFee.VolatilityReference,
	})
	return err
}

func upsertDammV2Position(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	position *meteora_damm_v2.PositionState,
) error {
	sql := `
		INSERT INTO sol_meteora_damm_v2_positions (
			address,
			pool,
			nft_mint,
			fee_a_per_token_checkpoint,
			fee_b_per_token_checkpoint,
			fee_a_pending,
			fee_b_pending,
			unlocked_liquidity,
			vested_liquidity,
			permanent_locked_liquidity,
			updated_at,
			created_at
		) VALUES (
			@address,
			@pool,
			@nft_mint,
			@fee_a_per_token_checkpoint,
			@fee_b_per_token_checkpoint,
			@fee_a_pending,
			@fee_b_pending,
			@unlocked_liquidity,
			@vested_liquidity,
			@permanent_locked_liquidity,
			NOW(),
			NOW()
		)
		ON CONFLICT (address) DO UPDATE SET
			pool = EXCLUDED.pool,
			nft_mint = EXCLUDED.nft_mint,
			fee_a_per_token_checkpoint = EXCLUDED.fee_a_per_token_checkpoint,
			fee_b_per_token_checkpoint = EXCLUDED.fee_b_per_token_checkpoint,
			fee_a_pending = EXCLUDED.fee_a_pending,
			fee_b_pending = EXCLUDED.fee_b_pending,
			unlocked_liquidity = EXCLUDED.unlocked_liquidity,
			vested_liquidity = EXCLUDED.vested_liquidity,
			permanent_locked_liquidity = EXCLUDED.permanent_locked_liquidity,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"address":                    account.String(),
		"pool":                       position.Pool.String(),
		"nft_mint":                   position.NftMint.String(),
		"fee_a_per_token_checkpoint": position.FeeAPerTokenCheckpoint,
		"fee_b_per_token_checkpoint": position.FeeBPerTokenCheckpoint,
		"fee_a_pending":              position.FeeAPending,
		"fee_b_pending":              position.FeeBPending,
		"unlocked_liquidity":         position.UnlockedLiquidity.BigInt(),
		"vested_liquidity":           position.VestedLiquidity.BigInt(),
		"permanent_locked_liquidity": position.PermanentLockedLiquidity.BigInt(),
	})
	return err
}

func upsertDammV2PositionMetrics(
	ctx context.Context,
	db database.DBTX,
	account solana.PublicKey,
	metrics *meteora_damm_v2.PositionMetrics,
) error {
	sql := `
		INSERT INTO sol_meteora_damm_v2_position_metrics (
			position,
			total_claimed_a_fee,
			total_claimed_b_fee,
			created_at,
			updated_at
		) VALUES (
			@position,
			@total_claimed_a_fee,
			@total_claimed_b_fee,
			NOW(),
			NOW()		 	
		)
		ON CONFLICT (position) DO UPDATE SET
			total_claimed_a_fee = EXCLUDED.total_claimed_a_fee,
			total_claimed_b_fee = EXCLUDED.total_claimed_b_fee,
			updated_at = NOW()
	`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"position":            account.String(),
		"total_claimed_a_fee": metrics.TotalClaimedAFee,
		"total_claimed_b_fee": metrics.TotalClaimedBFee,
	})
	return err
}
