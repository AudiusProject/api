package meteora_damm_v2

import (
	"math/big"
)

const LIQUIDITY_SCALE = 128

func GetUnclaimedFees(pool *Pool, position *PositionState) (*big.Int, *big.Int) {
	totalPositionLiquidity := big.NewInt(0).Add(
		big.NewInt(0).Add(position.UnlockedLiquidity.BigInt(), position.VestedLiquidity.BigInt()),
		position.PermanentLockedLiquidity.BigInt(),
	)

	feeA := big.NewInt(0).Sub(&pool.FeeAPerLiquidity.Int, &position.FeeAPerTokenCheckpoint.Int)
	feeB := big.NewInt(0).Sub(&pool.FeeBPerLiquidity.Int, &position.FeeBPerTokenCheckpoint.Int)

	feeA.Mul(feeA, totalPositionLiquidity)
	feeB.Mul(feeB, totalPositionLiquidity)
	feeA.Rsh(feeA, LIQUIDITY_SCALE)
	feeB.Rsh(feeB, LIQUIDITY_SCALE)

	feeA.Add(feeA, big.NewInt(0).SetUint64(position.FeeAPending))
	feeB.Add(feeB, big.NewInt(0).SetUint64(position.FeeBPending))

	return feeA, feeB
}
