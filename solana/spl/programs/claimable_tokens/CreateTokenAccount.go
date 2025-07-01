package claimable_tokens

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type CreateTokenAccount struct {
	EthAddress common.Address

	// Accounts
	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*CreateTokenAccount)(nil)
	_ solana.AccountsSettable = (*CreateTokenAccount)(nil)
	_ text.EncodableToTree    = (*CreateTokenAccount)(nil)
)

func NewCreateTokenAccountInstructionBuilder() *CreateTokenAccount {
	inst := &CreateTokenAccount{
		Accounts: make(solana.AccountMetaSlice, 7),
	}
	inst.Accounts[4] = solana.Meta(solana.SysVarRentPubkey)
	inst.Accounts[5] = solana.Meta(solana.TokenProgramID)
	inst.Accounts[6] = solana.Meta(solana.SystemProgramID)
	return inst
}

func (inst *CreateTokenAccount) SetEthAddress(ethAddress common.Address) *CreateTokenAccount {
	inst.EthAddress = ethAddress
	return inst
}

func (inst *CreateTokenAccount) SetPayer(payer solana.PublicKey) *CreateTokenAccount {
	inst.Accounts[0] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *CreateTokenAccount) Payer() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *CreateTokenAccount) SetMint(mint solana.PublicKey) *CreateTokenAccount {
	inst.Accounts[1] = solana.Meta(mint)
	return inst
}

func (inst *CreateTokenAccount) Mint() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *CreateTokenAccount) SetAuthority(authority solana.PublicKey) *CreateTokenAccount {
	inst.Accounts[2] = solana.Meta(authority)
	return inst
}

func (inst *CreateTokenAccount) Authority() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *CreateTokenAccount) SetUserBank(userBank solana.PublicKey) *CreateTokenAccount {
	inst.Accounts[3] = solana.Meta(userBank).WRITE()
	return inst
}

func (inst *CreateTokenAccount) UserBank() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *CreateTokenAccount) Validate() error {
	if inst.EthAddress.Big().Uint64() == uint64(0) {
		return errors.New("ethAddress not set")
	}

	if inst.Payer() == nil {
		return errors.New("payer not set")
	}

	if inst.Mint() == nil {
		return errors.New("mint not set")
	}

	if inst.Authority() == nil {
		return errors.New("authority not set")
	}

	if inst.UserBank() == nil {
		return errors.New("userBank not set")
	}

	return nil
}

func (inst *CreateTokenAccount) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_CreateTokenAccount),
	}}
}

func (inst CreateTokenAccount) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *CreateTokenAccount) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst *CreateTokenAccount) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *CreateTokenAccount) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("ClaimableTokens", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("CreateTokenAccount")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("EthAddress", inst.EthAddress))
					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("Payer", inst.Payer().PublicKey))
						accountsBranch.Child(format.Account("Mint", inst.Mint().PublicKey))
						accountsBranch.Child(format.Account("Authority", inst.Authority().PublicKey))
						accountsBranch.Child(format.Account("UserBank", inst.UserBank().PublicKey))
					})
				})
		})
}

func NewCreateTokenAccountInstruction(
	ethAddress common.Address,
	mint solana.PublicKey,
	payer solana.PublicKey,
) (*CreateTokenAccount, error) {
	authority, _, err := deriveAuthority(mint)
	if err != nil {
		return nil, err
	}
	userBank, err := deriveUserBankAccount(mint, ethAddress)
	if err != nil {
		return nil, err
	}
	return NewCreateTokenAccountInstructionBuilder().
			SetEthAddress(ethAddress).
			SetPayer(payer).
			SetMint(mint).
			SetAuthority(authority).
			SetUserBank(userBank),
		nil
}
