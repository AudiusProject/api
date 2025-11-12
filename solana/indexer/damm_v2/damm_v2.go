package damm_v2

import (
	"context"
	"fmt"
	"strings"

	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processDammV2Instruction(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	tx *solana.Transaction,
	instructionIndex int,
	instruction solana.CompiledInstruction,
	signature string,
	instLogger *zap.Logger,
) error {
	accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
	if err != nil {
		return fmt.Errorf("error resolving instruction accounts: %w", err)
	}

	inst, err := meteora_damm_v2.DecodeInstruction(accounts, instruction.Data)
	if err != nil {
		// Ignore unknown instruction types.
		// Not all DammV2 instruction types are implemented yet.
		// See: solana/spl/programs/meteora_damm_v2/instruction.go
		// See: https://github.com/gagliardetto/binary/blob/v0.8.0/variant.go#L315
		if strings.Contains(err.Error(), "no known type for type") {
			return nil // ignore unknown instruction types
		}
		return fmt.Errorf("error decoding meteora_damm_v2 instruction %d: %w", instructionIndex, err)
	}

	switch inst.TypeID {
	case meteora_damm_v2.InstructionImplDef.TypeID(meteora_damm_v2.Instruction_InitializeCustomPool):
		{
			if initializeCustomPoolInst, ok := inst.Impl.(*meteora_damm_v2.InitializeCustomPool); ok {
				err := insertDammV2InitializeCustomPool(
					ctx,
					db,
					slot,
					signature,
					instructionIndex,
					initializeCustomPoolInst,
				)
				if err != nil {
					return fmt.Errorf("error inserting meteora_damm_v2 initialize_custom_pool instruction %d: %w", instructionIndex, err)
				}
				instLogger.Info("damm_v2 initialize_custom_pool",
					zap.String("pool", initializeCustomPoolInst.GetPool().PublicKey.String()),
					zap.String("mint", initializeCustomPoolInst.GetTokenAMint().PublicKey.String()),
					zap.String("position", initializeCustomPoolInst.GetPosition().PublicKey.String()),
					zap.String("positionNFT", initializeCustomPoolInst.GetPositionNftAccount().PublicKey.String()),
					zap.String("positionNFTMint", initializeCustomPoolInst.GetPositionNftMint().PublicKey.String()),
				)
			}
		}
	}
	return nil
}

func insertDammV2InitializeCustomPool(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	signature string,
	instructionIndex int,
	inst *meteora_damm_v2.InitializeCustomPool,
) error {
	sql := `
    INSERT INTO sol_meteora_damm_v2_initialize_custom_pool_instructions (
        signature,
        instruction_index,
        slot,
        creator,
        position_nft_mint,
        position_nft_account,
        payer,
        pool_authority,
        pool,
        position,
        token_a_mint,
        token_b_mint,
        token_a_vault,
        token_b_vault,
        payer_token_a,
        payer_token_b,
        token_a_program,
        token_b_program,
        token_2022_program,
        system_program,
        event_authority,
        program,
        remaining_accounts,
        base_fee_cliff_fee_numerator,
        base_fee_first_factor,
        base_fee_second_factor_max_limiter_duration,
        base_fee_second_factor_max_fee_bps,
        base_fee_third_factor,
        base_fee_mode,
        dynamic_fee_bin_step,
        dynamic_fee_bin_step_u128,
        dynamic_fee_filter_period,
        dynamic_fee_decay_period,
        dynamic_fee_reduction_factor,
        dynamic_fee_max_volatility_accumulator,
        dynamic_fee_variable_fee_control,
        created_at,
        updated_at
    ) VALUES (
        @signature,
        @instruction_index,
        @slot,
        @creator,
        @position_nft_mint,
        @position_nft_account,
        @payer,
        @pool_authority,
        @pool,
        @position,
        @token_a_mint,
        @token_b_mint,
        @token_a_vault,
        @token_b_vault,
        @payer_token_a,
        @payer_token_b,
        @token_a_program,
        @token_b_program,
        @token_2022_program,
        @system_program,
        @event_authority,
        @program,
        @remaining_accounts,
        @base_fee_cliff_fee_numerator,
        @base_fee_first_factor,
		@base_fee_second_factor_max_limiter_duration,
        @base_fee_second_factor_max_fee_bps,
        @base_fee_third_factor,
        @base_fee_mode,
        @dynamic_fee_bin_step,
        @dynamic_fee_bin_step_u128,
        @dynamic_fee_filter_period,
        @dynamic_fee_decay_period,
        @dynamic_fee_reduction_factor,
        @dynamic_fee_max_volatility_accumulator,
        @dynamic_fee_variable_fee_control,
        NOW(),
        NOW()
    )
    ON CONFLICT DO NOTHING
    `

	rateLimiterParams, err := meteora_damm_v2.ParseRateLimiterSecondFactor(inst.PoolFees.BaseFee.SecondFactor[:])
	if err != nil {
		return fmt.Errorf("failed to parse rate limiter params: %w", err)
	}

	args := pgx.NamedArgs{
		"signature":            signature,
		"instruction_index":    instructionIndex,
		"slot":                 slot,
		"creator":              inst.GetCreator().PublicKey.String(),
		"position_nft_mint":    inst.GetPositionNftMint().PublicKey.String(),
		"position_nft_account": inst.GetPositionNftAccount().PublicKey.String(),
		"payer":                inst.GetPayer().PublicKey.String(),
		"pool_authority":       inst.GetPoolAuthority().PublicKey.String(),
		"pool":                 inst.GetPool().PublicKey.String(),
		"position":             inst.GetPosition().PublicKey.String(),
		"token_a_mint":         inst.GetTokenAMint().PublicKey.String(),
		"token_b_mint":         inst.GetTokenBMint().PublicKey.String(),
		"token_a_vault":        inst.GetTokenAVault().PublicKey.String(),
		"token_b_vault":        inst.GetTokenBVault().PublicKey.String(),
		"payer_token_a":        inst.GetPayerTokenA().PublicKey.String(),
		"payer_token_b":        inst.GetPayerTokenB().PublicKey.String(),
		"token_a_program":      inst.GetTokenAProgram().PublicKey.String(),
		"token_b_program":      inst.GetTokenBProgram().PublicKey.String(),
		"token_2022_program":   inst.GetToken2022Program().PublicKey.String(),
		"system_program":       inst.GetSystemProgram().PublicKey.String(),
		"event_authority":      inst.GetEventAuthority().PublicKey.String(),
		"program":              inst.GetProgram().PublicKey.String(),
		"remaining_accounts":   inst.RemainingAccounts(),

		"base_fee_cliff_fee_numerator":                inst.PoolFees.BaseFee.CliffFeeNumerator,
		"base_fee_first_factor":                       inst.PoolFees.BaseFee.FirstFactor,
		"base_fee_second_factor_max_limiter_duration": rateLimiterParams.MaxLimiterDuration,
		"base_fee_second_factor_max_fee_bps":          rateLimiterParams.MaxFeeBps,
		"base_fee_third_factor":                       inst.PoolFees.BaseFee.ThirdFactor,
		"base_fee_mode":                               inst.PoolFees.BaseFee.BaseFeeMode,
		"dynamic_fee_bin_step":                        inst.PoolFees.DynamicFee.BinStep,
		"dynamic_fee_bin_step_u128":                   inst.PoolFees.DynamicFee.BinStepU128.String(),
		"dynamic_fee_filter_period":                   inst.PoolFees.DynamicFee.FilterPeriod,
		"dynamic_fee_decay_period":                    inst.PoolFees.DynamicFee.DecayPeriod,
		"dynamic_fee_reduction_factor":                inst.PoolFees.DynamicFee.ReductionFactor,
		"dynamic_fee_max_volatility_accumulator":      inst.PoolFees.DynamicFee.MaxVolatilityAccumulator,
		"dynamic_fee_variable_fee_control":            inst.PoolFees.DynamicFee.VariableFeeControl,
	}

	_, err = db.Exec(ctx, sql, args)
	return err
}

func upsertDammV2Pool(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	account solana.PublicKey,
	pool *meteora_damm_v2.Pool,
) error {
	sqlPool := `
		INSERT INTO sol_meteora_damm_v2_pools (
			account,
			slot,
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
			version,
			fee_a_per_liquidity,
			fee_b_per_liquidity,
			permanent_lock_liquidity,
			creator,
			created_at,
			updated_at
		) VALUES (
			@account,
			@slot,
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
			@version,
			@fee_a_per_liquidity,
			@fee_b_per_liquidity,
			@permanent_lock_liquidity,
			@creator,
			NOW(),
			NOW()
		)
		ON CONFLICT (account) DO UPDATE SET
			slot = EXCLUDED.slot,
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
			version = EXCLUDED.version,
			fee_a_per_liquidity = EXCLUDED.fee_a_per_liquidity,
			fee_b_per_liquidity = EXCLUDED.fee_b_per_liquidity,
			permanent_lock_liquidity = EXCLUDED.permanent_lock_liquidity,
			creator = EXCLUDED.creator,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_pools.slot
	`
	args := pgx.NamedArgs{
		"account":                  account.String(),
		"slot":                     slot,
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
		"version":                  pool.Version,
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
	slot uint64,
	account solana.PublicKey,
	metrics *meteora_damm_v2.PoolMetrics,
) error {
	sqlMetrics := `
		INSERT INTO sol_meteora_damm_v2_pool_metrics (
			pool,
			slot,
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
			@slot,
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
			slot = EXCLUDED.slot,
			total_lp_a_fee = EXCLUDED.total_lp_a_fee,
			total_lp_b_fee = EXCLUDED.total_lp_b_fee,
			total_protocol_a_fee = EXCLUDED.total_protocol_a_fee,
			total_protocol_b_fee = EXCLUDED.total_protocol_b_fee,
			total_partner_a_fee = EXCLUDED.total_partner_a_fee,
			total_partner_b_fee = EXCLUDED.total_partner_b_fee,
			total_position = EXCLUDED.total_position,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_pool_metrics.slot
	`

	_, err := db.Exec(ctx, sqlMetrics, pgx.NamedArgs{
		"pool":                 account.String(),
		"slot":                 slot,
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
	slot uint64,
	account solana.PublicKey,
	fees *meteora_damm_v2.PoolFeesStruct,
) error {
	sqlFees := `
		INSERT INTO sol_meteora_damm_v2_pool_fees (
			pool,
			slot,
			partner_fee_percent,
			protocol_fee_percent,
			referral_fee_percent,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@slot,
			@partner_fee_percent,
			@protocol_fee_percent,
			@referral_fee_percent,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			slot = EXCLUDED.slot,
			partner_fee_percent = EXCLUDED.partner_fee_percent,
			protocol_fee_percent = EXCLUDED.protocol_fee_percent,
			referral_fee_percent = EXCLUDED.referral_fee_percent,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_pool_fees.slot
	`

	_, err := db.Exec(ctx, sqlFees, pgx.NamedArgs{
		"pool":                 account.String(),
		"slot":                 slot,
		"partner_fee_percent":  fees.PartnerFeePercent,
		"protocol_fee_percent": fees.ProtocolFeePercent,
		"referral_fee_percent": fees.ReferralFeePercent,
	})
	return err
}

func upsertDammV2PoolBaseFee(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	account solana.PublicKey,
	baseFee *meteora_damm_v2.BaseFeeStruct,
) error {
	sqlBaseFee := `
		INSERT INTO sol_meteora_damm_v2_pool_base_fees (
			pool,
			slot,
			cliff_fee_numerator,
			fee_scheduler_mode,
			number_of_period,
			period_frequency,
			reduction_factor,
			created_at,
			updated_at
		) VALUES (
			@pool,
			@slot,
			@cliff_fee_numerator,
			@fee_scheduler_mode,
			@number_of_period,
			@period_frequency,
			@reduction_factor,
			NOW(),
			NOW()
		)
		ON CONFLICT (pool) DO UPDATE SET
			slot = EXCLUDED.slot,
			cliff_fee_numerator = EXCLUDED.cliff_fee_numerator,
			fee_scheduler_mode = EXCLUDED.fee_scheduler_mode,
			number_of_period = EXCLUDED.number_of_period,
			period_frequency = EXCLUDED.period_frequency,
			reduction_factor = EXCLUDED.reduction_factor,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_pool_base_fees.slot
	`

	_, err := db.Exec(ctx, sqlBaseFee, pgx.NamedArgs{
		"pool":                account.String(),
		"slot":                slot,
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
	slot uint64,
	account solana.PublicKey,
	dynamicFee *meteora_damm_v2.DynamicFeeStruct,
) error {
	sqlDynamicFee := `
		INSERT INTO sol_meteora_damm_v2_pool_dynamic_fees (
			pool,
			slot,
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
			@slot,
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
			slot = EXCLUDED.slot,
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
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_pool_dynamic_fees.slot
	`

	_, err := db.Exec(ctx, sqlDynamicFee, pgx.NamedArgs{
		"pool":                       account.String(),
		"slot":                       slot,
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
	slot uint64,
	account solana.PublicKey,
	position *meteora_damm_v2.PositionState,
) error {
	sql := `
		INSERT INTO sol_meteora_damm_v2_positions (
			account,
			slot,
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
			@account,
			@slot,
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
		ON CONFLICT (account) DO UPDATE SET
			slot = EXCLUDED.slot,
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
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_positions.slot
	`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account":                    account.String(),
		"slot":                       slot,
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
	slot uint64,
	account solana.PublicKey,
	metrics *meteora_damm_v2.PositionMetrics,
) error {
	sql := `
		INSERT INTO sol_meteora_damm_v2_position_metrics (
			position,
			slot,
			total_claimed_a_fee,
			total_claimed_b_fee,
			created_at,
			updated_at
		) VALUES (
			@position,
			@slot,
			@total_claimed_a_fee,
			@total_claimed_b_fee,
			NOW(),
			NOW()		 	
		)
		ON CONFLICT (position) DO UPDATE SET
			slot = EXCLUDED.slot,
			total_claimed_a_fee = EXCLUDED.total_claimed_a_fee,
			total_claimed_b_fee = EXCLUDED.total_claimed_b_fee,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_damm_v2_position_metrics.slot
	`

	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"position":            account.String(),
		"slot":                slot,
		"total_claimed_a_fee": metrics.TotalClaimedAFee,
		"total_claimed_b_fee": metrics.TotalClaimedBFee,
	})
	return err
}
