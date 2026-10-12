package meteora_damm_v2

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type InitializeCustomPool struct {
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`

	PoolFees        PoolFeesParameters
	SqrtMinPrice    bin.Uint128
	SqrtMaxPrice    bin.Uint128
	HasAlphaVault   bool
	Liquidity       bin.Uint128
	SqrtPrice       bin.Uint128
	ActivationType  uint8
	CollectFeeMode  uint8
	ActivationPoint *uint64 `bin:"optional"`
}

func (inst *InitializeCustomPool) GetCreator() *solana.AccountMeta {
	return inst.Get(0)
}

func (inst *InitializeCustomPool) GetPositionNftMint() *solana.AccountMeta {
	return inst.Get(1)
}

func (inst *InitializeCustomPool) GetPositionNftAccount() *solana.AccountMeta {
	return inst.Get(2)
}

func (inst *InitializeCustomPool) GetPayer() *solana.AccountMeta {
	return inst.Get(3)
}

func (inst *InitializeCustomPool) GetPoolAuthority() *solana.AccountMeta {
	return inst.Get(4)
}

func (inst *InitializeCustomPool) GetPool() *solana.AccountMeta {
	return inst.Get(5)
}

func (inst *InitializeCustomPool) GetPosition() *solana.AccountMeta {
	return inst.Get(6)
}

func (inst *InitializeCustomPool) GetTokenAMint() *solana.AccountMeta {
	return inst.Get(7)
}

func (inst *InitializeCustomPool) GetTokenBMint() *solana.AccountMeta {
	return inst.Get(8)
}

func (inst *InitializeCustomPool) GetTokenAVault() *solana.AccountMeta {
	return inst.Get(9)
}

func (inst *InitializeCustomPool) GetTokenBVault() *solana.AccountMeta {
	return inst.Get(10)
}

func (inst *InitializeCustomPool) GetPayerTokenA() *solana.AccountMeta {
	return inst.Get(11)
}

func (inst *InitializeCustomPool) GetPayerTokenB() *solana.AccountMeta {
	return inst.Get(12)
}

func (inst *InitializeCustomPool) GetTokenAProgram() *solana.AccountMeta {
	return inst.Get(13)
}

func (inst *InitializeCustomPool) GetTokenBProgram() *solana.AccountMeta {
	return inst.Get(14)
}

func (inst *InitializeCustomPool) GetToken2022Program() *solana.AccountMeta {
	return inst.Get(15)
}

func (inst *InitializeCustomPool) GetSystemProgram() *solana.AccountMeta {
	return inst.Get(16)
}

func (inst *InitializeCustomPool) GetEventAuthority() *solana.AccountMeta {
	return inst.Get(17)
}

func (inst *InitializeCustomPool) GetProgram() *solana.AccountMeta {
	return inst.Get(18)
}

func (inst *InitializeCustomPool) RemainingAccounts() []*solana.AccountMeta {
	return inst.AccountMetaSlice[19:]
}
