package meteora_dbc_test

import (
	"encoding/base64"
	"math/big"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/meteora_dbc"
	bin "github.com/gagliardetto/binary"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func TestDecodePoolAccount(t *testing.T) {
	testData, err := base64.StdEncoding.DecodeString("1eAF0WJFd1wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADVACPQr7OAeCx6ocDwRH1mzY2ySBOSxODdpQc5kWP6afh33DUItBWZdLzRZEYcM2oLRbN13eBlK5O0vLxZGOjgoGiN7atsvEpANjsJ+9aoQjHXPIbqZUYXkAU+JaM7ioQ82X+lS+D7Ptu3dGKmZipPYi7SlADdIZEPRyOs502df69fWLDy5zSE/CDSdYWErv1+i5u4Zg1z6IL1vGzb13cB1CFCc7/IQMYfFaH5YBAAAAAAAAAAAAAM7sSw4CAAAAAAAAAAAAAADB2ZccBAAAAJ5h1C1D1VsBAAAAAAAAAAAlAaAVAAAAAAAAAAAAAAAAAAAAAAAAAADO7EsOAgAAAAAAAAAAAAAAeLMvOQgAAAAAAAAAAAAAAAAAAAAAAAAAt9mXHAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==")
	require.NoError(t, err)
	result := meteora_dbc.Pool{}
	dec := bin.NewBinDecoder(testData)
	err = dec.Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), result.VolatilityTracker.LastUpdateTimestamp)
	assert.Equal(t, bin.Uint128{}, result.VolatilityTracker.SqrtPriceReference)
	assert.Equal(t, bin.Uint128{}, result.VolatilityTracker.VolatilityAccumulator)
	assert.Equal(t, bin.Uint128{}, result.VolatilityTracker.VolatilityReference)

	assert.Equal(t, "ty4VYd4WZXVmhZsPVHLgqqR4eZbbntuRNqCNRh9pCWq", result.Config.String())
	assert.Equal(t, "Bjjet39ijcwurXHZTcoE37oX1Vjp6L5shBynyZBKuyAd", result.Creator.String())
	assert.Equal(t, "g8rfSkR5gfjKyvarWnP5zrnNjKhhhSHiYDbmvAgNao1", result.BaseMint.String())
	assert.Equal(t, "5Zg9Lqcdm5PmfuzkwMBTTFqHKNa2db4rW4eEPaGhmoZY", result.BaseVault.String())
	assert.Equal(t, "HteTDQFQFvTsGCm7tg5n9oRbaoa7pviAYUZp2NKfLb6o", result.QuoteVault.String())
	assert.Equal(t, uint64(902123156369850909), result.BaseReserve)
	assert.Equal(t, uint64(1744282775905), result.QuoteReserve)
	assert.Equal(t, uint64(0), result.ProtocolBaseFee)
	assert.Equal(t, uint64(8829791438), result.ProtocolQuoteFee)
	assert.Equal(t, uint64(0), result.PartnerBaseFee)
	assert.Equal(t, uint64(17659582913), result.PartnerQuoteFee)
	assert.Equal(t, big.NewInt(97906301427016094), result.SqrtPrice.BigInt())
	assert.Equal(t, uint64(362807589), result.ActivationPoint)
	assert.Equal(t, uint8(0), result.PoolType)
	assert.Equal(t, uint8(0), result.IsMigrated)
	assert.Equal(t, uint8(0), result.IsPartnerWithdrawSurplus)
	assert.Equal(t, uint8(0), result.IsProtocolWithdrawSurplus)
	assert.Equal(t, uint8(0), result.MigrationProgress)
	assert.Equal(t, uint8(0), result.IsWithdrawLeftover)
	assert.Equal(t, uint8(0), result.IsCreatorWithdrawSurplus)
	assert.Equal(t, uint8(0), result.MigrationFeeWithdrawStatus)

	assert.Equal(t, uint64(0), result.Metrics.TotalProtocolBaseFee)
	assert.Equal(t, uint64(8829791438), result.Metrics.TotalProtocolQuoteFee)
	assert.Equal(t, uint64(0), result.Metrics.TotalTradingBaseFee)
	assert.Equal(t, uint64(35319165816), result.Metrics.TotalTradingQuoteFee)

	assert.Equal(t, uint64(0), result.FinishCurveTimestamp)
	assert.Equal(t, uint64(0), result.CreatorBaseFee)
	assert.Equal(t, uint64(17659582903), result.CreatorQuoteFee)
}

func TestDecodePoolConfigAccount(t *testing.T) {
	testData, err := base64.StdEncoding.DecodeString("GmwOe3TmgSt7/DPMLnXBSHbMN5KDkE9JB3ZpESJXuzrf82mLYCJJQHMnYKWN/QLoMpzgYf1T6q8M/4URxhwIezYmnCD6QhNacydgpY39AugynOBh/VPqrwz/hRHGHAh7NiacIPpCE1qAlpgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAUFAABAAkAAAAyADIAAgEyAQAAAAAAAAAAAMe0sXK4lzAEwHwnGAIJAAAbP7eRoMO/Apf77//UaM8BAAAAAAAAAAAACL0TLfkAAAAAAAAAAAAAgFEBAAAAAAAhBwAAAAAAAAD4Gx0AAQAAAAAAAAAAAAAAAGSns7bgDQAAZKeztuANAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADvp8ZLN4lBAAAAAAAAAAAAc+byn+EOSgAAAAAAAAAAAACAr6nJ29qRT6Z5Ff0FAADL5mSoPrBTAAAAAAAAAAAAAKBXXBBfou2xE8+nvgYAAMU09pQykl4AAAAAAAAAAAAAwPZ0qwVtcXsKX/phCAAAOXJvrm3eagAAAAAAAAAAAADgkj4+ALMlz5d2xBsLAAAvV14JDMR4AAAAAAAAAAAAAEAfDSJV5BK/Ih8HgQ8AAF7HN/1JeIgAAAAAAAAAAAAAwGSYojcnEuk2OyuhFgAAPj4XE1A3mgAAAAAAAAAAAABg0KNYH25G0ZkHm2AiAADie1J5GUWuAAAAAAAAAAAAAEANCUCuThckqs3lKTYAAJyorm547sQAAAAAAAAAAAAAAJoiw+OkF2mFxnlEWAAAxTvQijyK3gAAAAAAAAAAAAAAKUHnqguxZrXJ7XSUAAD/lDhLf3r7AAAAAAAAAAAAAMBEQNMaMBNhPCqAOgEBABVZvN4bLhwBAAAAAAAAAAAAAIOOc3oBEBxzj5VzygEAnNQFz1ciQQEAAAAAAAAAAAAA3Dqd6FjsrbHInV1HAwAjfuniwuRqAQAAAAAAAAAAAICyq8/1gExpkbUs3SgGAD1M3WdVFZoBAAAAAAAAAAAAgFhFxuYoNeQy7EqR3AsAzk3w/9RozwEAAAAAAAAAAAAAjNW+i+q+0aYx45tkFwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==")
	require.NoError(t, err)
	result := meteora_dbc.PoolConfig{}
	dec := bin.NewBinDecoder(testData)
	err = dec.Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM", result.QuoteMint.String())
	assert.Equal(t, "8kWiEZFeuaPCanbJkwL4PvWDmx4zsLnRoXjUPBnvrLmX", result.FeeClaimer.String())
	assert.Equal(t, "8kWiEZFeuaPCanbJkwL4PvWDmx4zsLnRoXjUPBnvrLmX", result.LeftoverReceiver.String())

	assert.Equal(t, uint64(10000000), result.PoolFees.BaseFee.CliffFeeNumerator)
	assert.Equal(t, uint64(0), result.PoolFees.BaseFee.PeriodFrequency)
	assert.Equal(t, uint64(0), result.PoolFees.BaseFee.ReductionFactor)
	assert.Equal(t, uint16(0), result.PoolFees.BaseFee.NumberOfPeriod)
	assert.Equal(t, uint8(0), result.PoolFees.BaseFee.FeeSchedulerMode)

	assert.Equal(t, uint8(0), result.PoolFees.DynamicFee.Initialized)
	assert.Equal(t, uint32(0), result.PoolFees.DynamicFee.MaxVolatilityAccumulator)
	assert.Equal(t, uint32(0), result.PoolFees.DynamicFee.VariableFeeControl)
	assert.Equal(t, uint16(0), result.PoolFees.DynamicFee.BinStep)
	assert.Equal(t, uint16(0), result.PoolFees.DynamicFee.FilterPeriod)
	assert.Equal(t, uint16(0), result.PoolFees.DynamicFee.DecayPeriod)
	assert.Equal(t, uint16(0), result.PoolFees.DynamicFee.ReductionFactor)
	assert.Equal(t, bin.Uint128{}, result.PoolFees.DynamicFee.BinStepU128)

	assert.Equal(t, uint8(0), result.CollectFeeMode)
	assert.Equal(t, uint8(1), result.MigrationOption)
	assert.Equal(t, uint8(0), result.ActivationType)
	assert.Equal(t, uint8(9), result.TokenDecimal)
	assert.Equal(t, uint8(0), result.Version)
	assert.Equal(t, uint8(0), result.TokenType)
	assert.Equal(t, uint8(0), result.QuoteTokenFlag)
	assert.Equal(t, uint8(50), result.PartnerLockedLpPercentage)
	assert.Equal(t, uint8(0), result.PartnerLpPercentage)
	assert.Equal(t, uint8(50), result.CreatorLockedLpPercentage)
	assert.Equal(t, uint8(0), result.CreatorLpPercentage)
	assert.Equal(t, uint8(2), result.MigrationFeeOption)
	assert.Equal(t, uint8(1), result.FixedTokenSupplyFlag)
	assert.Equal(t, uint8(50), result.CreatorTradingFeePercentage)
	assert.Equal(t, uint8(1), result.TokenUpdateAuthority)
	assert.Equal(t, uint8(0), result.MigrationFeePercentage)
	assert.Equal(t, uint8(0), result.CreatorMigrationFeePercentage)
	assert.Equal(t, uint64(301907993487848647), result.SwapBaseAmount)
	assert.Equal(t, uint64(9904599825600), result.MigrationQuoteThreshold)
	assert.Equal(t, uint64(198092003034480411), result.MigrationBaseThreshold)
	assert.Equal(t, big.NewInt(130438178253306775), result.MigrationSqrtPrice.BigInt())

	assert.Equal(t, uint64(273972000000000), result.LockedVestingConfig.AmountPerPeriod)
	assert.Equal(t, uint64(0), result.LockedVestingConfig.CliffDurationFromMigrationTime)
	assert.Equal(t, uint64(86400), result.LockedVestingConfig.Frequency)
	assert.Equal(t, uint64(1825), result.LockedVestingConfig.NumberOfPeriod)
	assert.Equal(t, uint64(1100000000000), result.LockedVestingConfig.CliffUnlockAmount)

	assert.Equal(t, uint64(1000000000000000000), result.PreMigrationTokenSupply)
	assert.Equal(t, uint64(1000000000000000000), result.PostMigrationTokenSupply)
	assert.Equal(t, uint8(0), result.MigratedCollectFeeMode)
	assert.Equal(t, uint8(0), result.MigratedDynamicFee)
	assert.Equal(t, uint16(0), result.MigratedPoolFeeBps)
	assert.Equal(t, big.NewInt(18446744073709551), result.SqrtStartPrice.BigInt())

	assert.Len(t, result.Curve, 20)
	liq0, _ := big.NewInt(0).SetString("121463419384978290040000000000000", 10)
	assert.Equal(t, big.NewInt(20845510490515059), result.Curve[0].SqrtPrice.BigInt())
	assert.Equal(t, liq0, result.Curve[0].Liquidity.BigInt())

	liq15, _ := big.NewInt(0).SetString("121463419384978290040000000000000000", 10)
	assert.Equal(t, big.NewInt(130438178253327822), result.Curve[15].SqrtPrice.BigInt())
	assert.Equal(t, liq15, result.Curve[15].Liquidity.BigInt())
}
