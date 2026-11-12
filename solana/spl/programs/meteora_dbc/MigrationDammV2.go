package meteora_dbc

import "github.com/gagliardetto/solana-go"

type MigrationDammV2 struct {
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *MigrationDammV2) GetVirtualPool() *solana.AccountMeta {
	return inst.Get(0)
}

func (inst *MigrationDammV2) GetMigrationMetadata() *solana.AccountMeta {
	return inst.Get(1)
}

func (inst *MigrationDammV2) GetConfig() *solana.AccountMeta {
	return inst.Get(2)
}

func (inst *MigrationDammV2) GetDbcPoolAuthority() *solana.AccountMeta {
	return inst.Get(3)
}

func (inst *MigrationDammV2) GetDammV2Pool() *solana.AccountMeta {
	return inst.Get(4)
}

func (inst *MigrationDammV2) GetFirstPositionNftMint() *solana.AccountMeta {
	return inst.Get(5)
}

func (inst *MigrationDammV2) GetFirstPositionNftAccount() *solana.AccountMeta {
	return inst.Get(6)
}

func (inst *MigrationDammV2) GetFirstPosition() *solana.AccountMeta {
	return inst.Get(7)
}

func (inst *MigrationDammV2) GetSecondPositionNftMint() *solana.AccountMeta {
	return inst.Get(8)
}

func (inst *MigrationDammV2) GetSecondPositionNftAccount() *solana.AccountMeta {
	return inst.Get(9)
}

func (inst *MigrationDammV2) GetSecondPosition() *solana.AccountMeta {
	return inst.Get(10)
}

func (inst *MigrationDammV2) GetDammV2PoolAuthority() *solana.AccountMeta {
	return inst.Get(11)
}

func (inst *MigrationDammV2) GetAmmProgram() *solana.AccountMeta {
	return inst.Get(12)
}

func (inst *MigrationDammV2) GetBaseMint() *solana.AccountMeta {
	return inst.Get(13)
}

func (inst *MigrationDammV2) GetQuoteMint() *solana.AccountMeta {
	return inst.Get(14)
}

func (inst *MigrationDammV2) GetTokenAVault() *solana.AccountMeta {
	return inst.Get(15)
}

func (inst *MigrationDammV2) GetTokenBVault() *solana.AccountMeta {
	return inst.Get(16)
}

func (inst *MigrationDammV2) GetBaseVault() *solana.AccountMeta {
	return inst.Get(17)
}

func (inst *MigrationDammV2) GetQuoteVault() *solana.AccountMeta {
	return inst.Get(18)
}

func (inst *MigrationDammV2) GetPayer() *solana.AccountMeta {
	return inst.Get(19)
}

func (inst *MigrationDammV2) GetTokenBaseProgram() *solana.AccountMeta {
	return inst.Get(20)
}

func (inst *MigrationDammV2) GetTokenQuoteProgram() *solana.AccountMeta {
	return inst.Get(21)
}

func (inst *MigrationDammV2) GetToken2022Program() *solana.AccountMeta {
	return inst.Get(22)
}

func (inst *MigrationDammV2) GetDammEventAuthority() *solana.AccountMeta {
	return inst.Get(23)
}

func (inst *MigrationDammV2) GetSystemProgram() *solana.AccountMeta {
	return inst.Get(24)
}
