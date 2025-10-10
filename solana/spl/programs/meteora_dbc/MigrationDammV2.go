package meteora_dbc

import "github.com/gagliardetto/solana-go"

type MigrationDammV2 struct {
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func (inst *MigrationDammV2) GetVirtualPool() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst *MigrationDammV2) GetMigrationMetadata() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

func (inst *MigrationDammV2) GetConfig() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst *MigrationDammV2) GetPoolAuthority() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst *MigrationDammV2) GetPool() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(4)
}

func (inst *MigrationDammV2) GetFirstPositionNftMint() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(5)
}

func (inst *MigrationDammV2) GetFirstPositionNftAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(6)
}

func (inst *MigrationDammV2) GetFirstPosition() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(7)
}

func (inst *MigrationDammV2) GetSecondPositionNftMint() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(8)
}

func (inst *MigrationDammV2) GetSecondPositionNftAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(9)
}

func (inst *MigrationDammV2) GetSecondPosition() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(10)
}

func (inst *MigrationDammV2) GetDammPoolAuthority() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(11)
}

func (inst *MigrationDammV2) GetAmmProgram() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(12)
}

func (inst *MigrationDammV2) GetBaseMint() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(13)
}

func (inst *MigrationDammV2) GetQuoteMint() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(14)
}

func (inst *MigrationDammV2) GetTokenAVault() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(15)
}

func (inst *MigrationDammV2) GetTokenBVault() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(16)
}

func (inst *MigrationDammV2) GetBaseVault() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(17)
}

func (inst *MigrationDammV2) GetQuoteVault() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(18)
}

func (inst *MigrationDammV2) GetPayer() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(19)
}

func (inst *MigrationDammV2) GetTokenBaseProgram() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(20)
}

func (inst *MigrationDammV2) GetTokenQuoteProgram() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(21)
}

func (inst *MigrationDammV2) GetToken2022Program() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(22)
}

func (inst *MigrationDammV2) GetDammEventAuthority() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(23)
}

func (inst *MigrationDammV2) GetSystemProgram() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(24)
}
