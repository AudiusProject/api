package dbc

import (
	"context"

	"api.audius.co/solana/spl/programs/meteora_dbc"
	"github.com/jackc/pgx/v5"
)

func upsertDbcConfig(
	ctx context.Context,
	tx pgx.Tx,
	slot uint64,
	account string,
	config *meteora_dbc.PoolConfig,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sol_meteora_dbc_configs (
			account,
			slot,
			quote_mint,
			fee_claimer,
			leftover_receiver,
			collect_fee_mode,
			migration_option,
			activation_type,
			token_decimal,
			version,
			token_type,
			quote_token_flag,
			partner_locked_lp_percentage,
			partner_lp_percentage,
			creator_locked_lp_percentage,
			creator_lp_percentage,
			migration_fee_option,
			fixed_token_supply_flag,
			creator_trading_fee_percentage,
			token_update_authority,
			migration_fee_percentage,
			creator_migration_fee_percentage,
			swap_base_amount,
			migration_quote_threshold,
			migration_base_threshold,
			migration_sqrt_price,
			pre_migration_token_supply,
			post_migration_token_supply,
			migrated_collect_fee_mode,
			migrated_dynamic_fee,
			migrated_pool_fee_bps,
			sqrt_start_price,
			curve,
			created_at,
			updated_at
		) VALUES (
			@account,
			@slot,
			@quote_mint,
			@fee_claimer,
			@leftover_receiver,
			@collect_fee_mode,
			@migration_option,
			@activation_type,
			@token_decimal,
			@version,
			@token_type,
			@quote_token_flag,
			@partner_locked_lp_percentage,
			@partner_lp_percentage,
			@creator_locked_lp_percentage,
			@creator_lp_percentage,
			@migration_fee_option,
			@fixed_token_supply_flag,
			@creator_trading_fee_percentage,
			@token_update_authority,
			@migration_fee_percentage,
			@creator_migration_fee_percentage,
			@swap_base_amount,
			@migration_quote_threshold,
			@migration_base_threshold,
			@migration_sqrt_price,
			@pre_migration_token_supply,
			@post_migration_token_supply,
			@migrated_collect_fee_mode,
			@migrated_dynamic_fee,
			@migrated_pool_fee_bps,
			@sqrt_start_price,
			@curve,
			NOW(),
			NOW()
		)
		ON CONFLICT (account) DO UPDATE
		SET
			slot = EXCLUDED.slot,
			quote_mint = EXCLUDED.quote_mint,
			fee_claimer = EXCLUDED.fee_claimer,
			leftover_receiver = EXCLUDED.leftover_receiver,
			collect_fee_mode = EXCLUDED.collect_fee_mode,
			migration_option = EXCLUDED.migration_option,
			activation_type = EXCLUDED.activation_type,
			token_decimal = EXCLUDED.token_decimal,
			version = EXCLUDED.version,
			token_type = EXCLUDED.token_type,
			quote_token_flag = EXCLUDED.quote_token_flag,
			partner_locked_lp_percentage = EXCLUDED.partner_locked_lp_percentage,
			partner_lp_percentage = EXCLUDED.partner_lp_percentage,
			creator_locked_lp_percentage = EXCLUDED.creator_locked_lp_percentage,
			creator_lp_percentage = EXCLUDED.creator_lp_percentage,
			migration_fee_option = EXCLUDED.migration_fee_option,
			fixed_token_supply_flag = EXCLUDED.fixed_token_supply_flag,
			creator_trading_fee_percentage = EXCLUDED.creator_trading_fee_percentage,
			token_update_authority = EXCLUDED.token_update_authority,
			migration_fee_percentage = EXCLUDED.migration_fee_percentage,
			creator_migration_fee_percentage = EXCLUDED.creator_migration_fee_percentage,
			swap_base_amount = EXCLUDED.swap_base_amount,
			migration_quote_threshold = EXCLUDED.migration_quote_threshold,
			migration_base_threshold = EXCLUDED.migration_base_threshold,
			migration_sqrt_price = EXCLUDED.migration_sqrt_price,
			pre_migration_token_supply = EXCLUDED.pre_migration_token_supply,
			post_migration_token_supply = EXCLUDED.post_migration_token_supply,
			migrated_collect_fee_mode = EXCLUDED.migrated_collect_fee_mode,
			migrated_dynamic_fee = EXCLUDED.migrated_dynamic_fee,
			migrated_pool_fee_bps = EXCLUDED.migrated_pool_fee_bps,
			sqrt_start_price = EXCLUDED.sqrt_start_price,
			curve = EXCLUDED.curve,
			updated_at = NOW()
		WHERE EXCLUDED.slot > sol_meteora_dbc_configs.slot
	`, pgx.NamedArgs{
		"account":                          account,
		"slot":                             slot,
		"quote_mint":                       config.QuoteMint.String(),
		"fee_claimer":                      config.FeeClaimer.String(),
		"leftover_receiver":                config.LeftoverReceiver.String(),
		"collect_fee_mode":                 config.CollectFeeMode,
		"migration_option":                 config.MigrationOption,
		"activation_type":                  config.ActivationType,
		"token_decimal":                    config.TokenDecimal,
		"version":                          config.Version,
		"token_type":                       config.TokenType,
		"quote_token_flag":                 config.QuoteTokenFlag,
		"partner_locked_lp_percentage":     config.PartnerLockedLpPercentage,
		"partner_lp_percentage":            config.PartnerLpPercentage,
		"creator_locked_lp_percentage":     config.CreatorLockedLpPercentage,
		"creator_lp_percentage":            config.CreatorLpPercentage,
		"migration_fee_option":             config.MigrationFeeOption,
		"fixed_token_supply_flag":          config.FixedTokenSupplyFlag,
		"creator_trading_fee_percentage":   config.CreatorTradingFeePercentage,
		"token_update_authority":           config.TokenUpdateAuthority,
		"migration_fee_percentage":         config.MigrationFeePercentage,
		"creator_migration_fee_percentage": config.CreatorMigrationFeePercentage,
		"swap_base_amount":                 config.SwapBaseAmount,
		"migration_quote_threshold":        config.MigrationQuoteThreshold,
		"migration_base_threshold":         config.MigrationBaseThreshold,
		"migration_sqrt_price":             config.MigrationSqrtPrice.String(),
		"pre_migration_token_supply":       config.PreMigrationTokenSupply,
		"post_migration_token_supply":      config.PostMigrationTokenSupply,
		"migrated_collect_fee_mode":        config.MigratedCollectFeeMode,
		"migrated_dynamic_fee":             config.MigratedDynamicFee,
		"migrated_pool_fee_bps":            config.MigratedPoolFeeBps,
		"sqrt_start_price":                 config.SqrtStartPrice.String(),
		"curve":                            config.Curve,
	})
	if err != nil {
		return err
	}
	return nil
}

func upsertDbcConfigFees(
	ctx context.Context,
	tx pgx.Tx,
	slot uint64,
	account string,
	fees *meteora_dbc.PoolFeesConfig,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sol_meteora_dbc_config_fees (
			config,
			slot,
			base_fee_cliff_fee_numerator,
			base_fee_period_frequency,
			base_fee_reduction_factor,
			base_fee_number_of_period,
			base_fee_fee_scheduler_mode,
			dynamic_fee_initialized,
			dynamic_fee_max_volatility_accumulator,
			dynamic_fee_variable_fee_control,
			dynamic_fee_bin_step,
			dynamic_fee_filter_period,
			dynamic_fee_decay_period,
			dynamic_fee_reduction_factor,
			dynamic_fee_bin_step_u128,
			protocol_fee_percent,
			referral_fee_percent
		) VALUES (
			@config,
			@slot,
			@base_fee_cliff_fee_numerator,
			@base_fee_period_frequency,
			@base_fee_reduction_factor,
			@base_fee_number_of_period,
			@base_fee_fee_scheduler_mode,
			@dynamic_fee_initialized,
			@dynamic_fee_max_volatility_accumulator,
			@dynamic_fee_variable_fee_control,
			@dynamic_fee_bin_step,
			@dynamic_fee_filter_period,
			@dynamic_fee_decay_period,
			@dynamic_fee_reduction_factor,
			@dynamic_fee_bin_step_u128,
			@protocol_fee_percent,
			@referral_fee_percent
		)
		ON CONFLICT (config) DO UPDATE
		SET
			slot = EXCLUDED.slot,
			base_fee_cliff_fee_numerator = EXCLUDED.base_fee_cliff_fee_numerator,
			base_fee_period_frequency = EXCLUDED.base_fee_period_frequency,
			base_fee_reduction_factor = EXCLUDED.base_fee_reduction_factor,
			base_fee_number_of_period = EXCLUDED.base_fee_number_of_period,
			base_fee_fee_scheduler_mode = EXCLUDED.base_fee_fee_scheduler_mode,
			dynamic_fee_initialized = EXCLUDED.dynamic_fee_initialized,
			dynamic_fee_max_volatility_accumulator = EXCLUDED.dynamic_fee_max_volatility_accumulator,
			dynamic_fee_variable_fee_control = EXCLUDED.dynamic_fee_variable_fee_control,
			dynamic_fee_bin_step = EXCLUDED.dynamic_fee_bin_step,
			dynamic_fee_filter_period = EXCLUDED.dynamic_fee_filter_period,
			dynamic_fee_decay_period = EXCLUDED.dynamic_fee_decay_period,
			dynamic_fee_reduction_factor = EXCLUDED.dynamic_fee_reduction_factor,
			dynamic_fee_bin_step_u128 = EXCLUDED.dynamic_fee_bin_step_u128,
			protocol_fee_percent = EXCLUDED.protocol_fee_percent,
			referral_fee_percent = EXCLUDED.referral_fee_percent
		WHERE EXCLUDED.slot > sol_meteora_dbc_config_fees.slot
	`, pgx.NamedArgs{
		"config":                                 account,
		"slot":                                   slot,
		"base_fee_cliff_fee_numerator":           fees.BaseFee.CliffFeeNumerator,
		"base_fee_period_frequency":              fees.BaseFee.PeriodFrequency,
		"base_fee_reduction_factor":              fees.BaseFee.ReductionFactor,
		"base_fee_number_of_period":              fees.BaseFee.NumberOfPeriod,
		"base_fee_fee_scheduler_mode":            fees.BaseFee.FeeSchedulerMode,
		"dynamic_fee_initialized":                fees.DynamicFee.Initialized,
		"dynamic_fee_max_volatility_accumulator": fees.DynamicFee.MaxVolatilityAccumulator,
		"dynamic_fee_variable_fee_control":       fees.DynamicFee.VariableFeeControl,
		"dynamic_fee_bin_step":                   fees.DynamicFee.BinStep,
		"dynamic_fee_filter_period":              fees.DynamicFee.FilterPeriod,
		"dynamic_fee_decay_period":               fees.DynamicFee.DecayPeriod,
		"dynamic_fee_reduction_factor":           fees.DynamicFee.ReductionFactor,
		"dynamic_fee_bin_step_u128":              fees.DynamicFee.BinStepU128.String(),
		"protocol_fee_percent":                   fees.ProtocolFeePercent,
		"referral_fee_percent":                   fees.ReferralFeePercent,
	})
	if err != nil {
		return err
	}
	return nil
}

func upsertDbcConfigVesting(
	ctx context.Context,
	tx pgx.Tx,
	slot uint64,
	account string,
	vesting *meteora_dbc.LockedVestingConfig,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sol_meteora_dbc_config_vestings (
			config,
			slot,
			amount_per_period,
			cliff_duration_from_migration_time,
			frequency,
			number_of_period,
			cliff_unlock_amount
		) VALUES (
			@config,
			@slot,
			@amount_per_period,
			@cliff_duration_from_migration_time,
			@frequency,
			@number_of_period,
			@cliff_unlock_amount
		)
		ON CONFLICT (config) DO UPDATE
		SET
			slot = EXCLUDED.slot,
			amount_per_period = EXCLUDED.amount_per_period,
			cliff_duration_from_migration_time = EXCLUDED.cliff_duration_from_migration_time,
			frequency = EXCLUDED.frequency,
			number_of_period = EXCLUDED.number_of_period,
			cliff_unlock_amount = EXCLUDED.cliff_unlock_amount
		WHERE EXCLUDED.slot > sol_meteora_dbc_config_vestings.slot
	`, pgx.NamedArgs{
		"config":                             account,
		"slot":                               slot,
		"amount_per_period":                  vesting.AmountPerPeriod,
		"cliff_duration_from_migration_time": vesting.CliffDurationFromMigrationTime,
		"frequency":                          vesting.Frequency,
		"number_of_period":                   vesting.NumberOfPeriod,
		"cliff_unlock_amount":                vesting.CliffUnlockAmount,
	})
	if err != nil {
		return err
	}
	return nil
}
