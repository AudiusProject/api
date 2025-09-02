package meteora_dbc

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type BaseFeeConfig struct {
	CliffFeeNumerator uint64
	PeriodFrequency   uint64
	ReductionFactor   uint64
	NumberOfPeriod    uint16
	FeeSchedulerMode  uint8
	Padding0          [5]uint8
}

type DynamicFeeConfig struct {
	Initialized              uint8
	Padding                  [7]uint8
	MaxVolatilityAccumulator uint32
	VariableFeeControl       uint32
	BinStep                  uint16
	FilterPeriod             uint16
	DecayPeriod              uint16
	ReductionFactor          uint16
	Padding2                 [8]uint8
	BinStepU128              bin.Uint128
}

type PoolFeesConfig struct {
	BaseFee            BaseFeeConfig
	DynamicFee         DynamicFeeConfig
	Padding0           [5]uint64
	Padding1           [6]uint8
	ProtocolFeePercent uint8
	ReferralFeePercent uint8
}

type LiquidityDistributionConfig struct {
	SqrtPrice bin.Uint128
	Liquidity bin.Uint128
}

type LockedVestingConfig struct {
	AmountPerPeriod                uint64
	CliffDurationFromMigrationTime uint64
	Frequency                      uint64
	NumberOfPeriod                 uint64
	CliffUnlockAmount              uint64
	Padding                        uint64
}

type PoolConfig struct {
	Discriminator                 [8]uint8
	QuoteMint                     solana.PublicKey
	FeeClaimer                    solana.PublicKey
	LeftoverReceiver              solana.PublicKey
	PoolFees                      PoolFeesConfig
	CollectFeeMode                uint8
	MigrationOption               uint8
	ActivationType                uint8
	TokenDecimal                  uint8
	Version                       uint8
	TokenType                     uint8
	QuoteTokenFlag                uint8
	PartnerLockedLpPercentage     uint8
	PartnerLpPercentage           uint8
	CreatorLockedLpPercentage     uint8
	CreatorLpPercentage           uint8
	MigrationFeeOption            uint8
	FixedTokenSupplyFlag          uint8
	CreatorTradingFeePercentage   uint8
	TokenUpdateAuthority          uint8
	MigrationFeePercentage        uint8
	CreatorMigrationFeePercentage uint8
	Padding0                      [7]uint8
	SwapBaseAmount                uint64
	MigrationQuoteThreshold       uint64
	MigrationBaseThreshold        uint64
	MigrationSqrtPrice            bin.Uint128
	LockedVestingConfig           LockedVestingConfig
	PreMigrationTokenSupply       uint64
	PostMigrationTokenSupply      uint64
	MigratedCollectFeeMode        uint8
	MigratedDynamicFee            uint8
	MigratedPoolFeeBps            uint16
	Padding1                      [12]uint8
	Padding2                      bin.Uint128
	SqrtStartPrice                bin.Uint128
	Curve                         [20]LiquidityDistributionConfig
}

type VolatilityTracker struct {
	LastUpdateTimestamp   uint64
	Padding               [8]uint8
	SqrtPriceReference    bin.Uint128
	VolatilityAccumulator bin.Uint128
	VolatilityReference   bin.Uint128
}

type PoolMetrics struct {
	TotalProtocolBaseFee  uint64
	TotalProtocolQuoteFee uint64
	TotalTradingBaseFee   uint64
	TotalTradingQuoteFee  uint64
}

type Pool struct {
	Discriminator              [8]uint8
	VolatilityTracker          VolatilityTracker
	Config                     solana.PublicKey
	Creator                    solana.PublicKey
	BaseMint                   solana.PublicKey
	BaseVault                  solana.PublicKey
	QuoteVault                 solana.PublicKey
	BaseReserve                uint64
	QuoteReserve               uint64
	ProtocolBaseFee            uint64
	ProtocolQuoteFee           uint64
	PartnerBaseFee             uint64
	PartnerQuoteFee            uint64
	SqrtPrice                  bin.Uint128
	ActivationPoint            uint64
	PoolType                   uint8
	IsMigrated                 uint8
	IsPartnerWithdrawSurplus   uint8
	IsProtocolWithdrawSurplus  uint8
	MigrationProgress          uint8
	IsWithdrawLeftover         uint8
	IsCreatorWithdrawSurplus   uint8
	MigrationFeeWithdrawStatus uint8
	Metrics                    PoolMetrics
	FinishCurveTimestamp       uint64
	CreatorBaseFee             uint64
	CreatorQuoteFee            uint64
	Padding1                   [7]uint64
}
