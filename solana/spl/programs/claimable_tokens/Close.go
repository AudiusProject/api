package claimable_tokens

import (
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type Close struct {
	EthAddress common.Address

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*Close)(nil)
	_ solana.AccountsSettable = (*Close)(nil)
	_ text.EncodableToTree    = (*Close)(nil)
)

func NewCloseInstructionBuilder() *Close {
	inst := &Close{
		AccountMetaSlice: make(solana.AccountMetaSlice, 4),
	}
	inst.AccountMetaSlice[3] = solana.NewAccountMeta(solana.TokenProgramID, false, false)
	return inst
}

func (inst *Close) SetEthAddress(ethAddress common.Address) *Close {
	inst.EthAddress = ethAddress
	return inst
}

func (inst *Close) UserBank() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst *Close) SetUserBank(userBank solana.PublicKey) *Close {
	inst.AccountMetaSlice[0] = solana.NewAccountMeta(userBank, true, false)
	return inst
}

func (inst *Close) Authority() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

func (inst *Close) SetAuthority(authority solana.PublicKey) *Close {
	inst.AccountMetaSlice[1] = solana.NewAccountMeta(authority, true, false)
	return inst
}

func (inst *Close) Destination() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst *Close) SetDestination(destination solana.PublicKey) *Close {
	inst.AccountMetaSlice[2] = solana.NewAccountMeta(destination, true, false)
	return inst
}

func (inst *Close) TokenProgram() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst *Close) SetTokenProgram(tokenProgram solana.PublicKey) *Close {
	inst.AccountMetaSlice[3] = solana.NewAccountMeta(tokenProgram, false, false)
	return inst
}

func (inst *Close) CloseAuthority() *solana.AccountMeta {
	if len(inst.AccountMetaSlice) < 5 {
		return nil
	}
	return inst.AccountMetaSlice.Get(4)
}

func (inst *Close) SetCloseAuthority(closeAuthority solana.PublicKey) *Close {
	if len(inst.AccountMetaSlice) < 5 {
		inst.AccountMetaSlice = append(inst.AccountMetaSlice, solana.NewAccountMeta(closeAuthority, true, true))
		return inst
	}
	inst.AccountMetaSlice[4] = solana.NewAccountMeta(closeAuthority, true, true)
	return inst
}

func (inst *Close) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_Close),
	}}
}

// ----- text.EncodableToTree Implementation -----

func (inst *Close) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("ClaimableTokens", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("Close")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("EthAddress", inst.EthAddress))
					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("UserBank", inst.UserBank().PublicKey))
						accountsBranch.Child(format.Account("Authority", inst.Authority().PublicKey))
						accountsBranch.Child(format.Account("Destination", inst.Destination().PublicKey))
						accountsBranch.Child(format.Account("TokenProgram", inst.TokenProgram().PublicKey))
						if inst.CloseAuthority() != nil {
							accountsBranch.Child(format.Account("CloseAuthority", inst.CloseAuthority().PublicKey))
						}
					})
				})
		})
}
