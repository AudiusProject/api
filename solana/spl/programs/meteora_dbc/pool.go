package meteora_dbc

import (
	"math/big"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

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

func (p Pool) GetMigrationProgress(migrationQuoteThreshold uint64) float64 {

	quoteReserve := new(big.Int).SetUint64(p.QuoteReserve)
	migrationQuoteThresholdBig := new(big.Int).SetUint64(migrationQuoteThreshold)
	quotient := new(big.Rat).SetFrac(quoteReserve, migrationQuoteThresholdBig)
	progress, _ := quotient.Float64()

	return progress
}

func (p Pool) GetQuotePrice(tokenBaseDecimals int, tokenQuoteDecimals int) float64 {
	sqrtPrice := p.SqrtPrice.BigInt()
	sqrtPriceSquared := new(big.Int).Mul(sqrtPrice, sqrtPrice)
	decimalsFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenBaseDecimals-tokenQuoteDecimals)), nil)

	numerator := new(big.Int).Mul(sqrtPriceSquared, decimalsFactor)
	divisor := new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil)
	quotient := new(big.Rat).SetFrac(numerator, divisor)

	price, _ := quotient.Float64()

	return price
}
