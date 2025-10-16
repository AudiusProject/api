package meteora_damm_v2

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type BaseFeeStruct struct {
	CliffFeeNumerator uint64
	FeeSchedulerMode  uint8
	Padding0          [5]uint8
	NumberOfPeriod    uint16
	PeriodFrequency   uint64
	ReductionFactor   uint64
	Padding1          uint64
}

type DynamicFeeStruct struct {
	Initialized              uint8
	Padding                  [7]uint8
	MaxVolatilityAccumulator uint32
	VariableFeeControl       uint32
	BinStep                  uint16
	FilterPeriod             uint16
	DecayPeriod              uint16
	ReductionFactor          uint16
	LastUpdateTimestamp      uint64
	BinStepU128              bin.Uint128
	SqrtPriceReference       bin.Uint128
	VolatilityAccumulator    bin.Uint128
	VolatilityReference      bin.Uint128
}

type PoolFeesStruct struct {
	BaseFee            BaseFeeStruct
	ProtocolFeePercent uint8
	PartnerFeePercent  uint8
	ReferralFeePercent uint8
	Padding0           [5]uint8
	DynamicFee         DynamicFeeStruct
	Padding1           [2]uint64
}

type PoolMetrics struct {
	TotalLpAFee       bin.Uint128
	TotalLpBFee       bin.Uint128
	TotalProtocolAFee uint64
	TotalProtocolBFee uint64
	TotalPartnerAFee  uint64
	TotalPartnerBFee  uint64
	TotalPosition     uint64
	Padding           uint64
}

type RewardInfo struct {
	Initialized                         uint8
	RewardTokenFlag                     uint8
	Padding0                            [6]uint8
	Padding1                            [8]uint8
	Mint                                solana.PublicKey
	Vault                               solana.PublicKey
	Funder                              solana.PublicKey
	RewardDuration                      uint64
	RewardDurationEnd                   uint64
	RewardRate                          bin.Uint128
	RewardPerTokenStored                [32]uint8
	LastUpdateTime                      uint64
	CumulativeSecondsWithEmptyLiquidity uint64
}

type Pool struct {
	Discriminator          [8]uint8
	PoolFees               PoolFeesStruct
	TokenAMint             solana.PublicKey
	TokenBMint             solana.PublicKey
	TokenAVault            solana.PublicKey
	TokenBVault            solana.PublicKey
	WhitelistedVault       solana.PublicKey
	Partner                solana.PublicKey
	Liquidity              bin.Uint128
	Padding                bin.Uint128
	ProtocolAFee           uint64
	ProtocolBFee           uint64
	PartnerAFee            uint64
	PartnerBFee            uint64
	SqrtMinPrice           bin.Uint128
	SqrtMaxPrice           bin.Uint128
	SqrtPrice              bin.Uint128
	ActivationPoint        uint64
	ActivationType         uint8
	PoolStatus             uint8
	TokenAFlag             uint8
	TokenBFlag             uint8
	CollectFeeMode         uint8
	PoolType               uint8
	Version                uint8
	Padding0               uint8
	FeeAPerLiquidity       Uint256LE
	FeeBPerLiquidity       Uint256LE
	PermanentLockLiquidity bin.Uint128
	Metrics                PoolMetrics
	Creator                solana.PublicKey
	Padding1               [6]uint64
	RewardInfos            [2]RewardInfo
}

type PositionMetrics struct {
	TotalClaimedAFee uint64
	TotalClaimedBFee uint64
}

type UserRewardInfo struct {
	RewardPerTokenCheckpoint [32]uint8
	RewardPendings           uint64
	TotalClaimedRewards      uint64
}

type PositionState struct {
	Discriminator            [8]uint8
	Pool                     solana.PublicKey
	NftMint                  solana.PublicKey
	FeeAPerTokenCheckpoint   Uint256LE
	FeeBPerTokenCheckpoint   Uint256LE
	FeeAPending              uint64
	FeeBPending              uint64
	UnlockedLiquidity        bin.Uint128
	VestedLiquidity          bin.Uint128
	PermanentLockedLiquidity bin.Uint128
	Metrics                  PositionMetrics
	RewardInfos              [2]UserRewardInfo
	Padding                  [6]bin.Uint128
}
