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

var (
	_ solana.AccountsGettable = (*Transfer)(nil)
	_ solana.AccountsSettable = (*Transfer)(nil)
	_ text.EncodableToTree    = (*Transfer)(nil)
)

type Transfer struct {
	SenderEthAddress common.Address

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

type SignedTransferData struct {
	Destination solana.PublicKey
	Amount      uint64
	Nonce       uint64
}

func NewTransferInstructionBuilder() *Transfer {
	inst := &Transfer{
		Accounts: make(solana.AccountMetaSlice, 9),
	}
	inst.Accounts[5] = solana.Meta(solana.SysVarRentPubkey)
	inst.Accounts[6] = solana.Meta(solana.SysVarInstructionsPubkey)
	inst.Accounts[7] = solana.Meta(solana.SystemProgramID)
	inst.Accounts[8] = solana.Meta(solana.TokenProgramID)
	return inst
}

func (inst *Transfer) SetSenderEthAddress(ethAddress common.Address) *Transfer {
	inst.SenderEthAddress = ethAddress
	return inst
}

func (inst *Transfer) SetPayer(payer solana.PublicKey) *Transfer {
	inst.Accounts[0] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *Transfer) Payer() *solana.AccountMeta {
	return inst.Accounts[0]
}

func (inst *Transfer) SetSenderUserBank(userBank solana.PublicKey) *Transfer {
	inst.Accounts[1] = solana.Meta(userBank).WRITE()
	return inst
}

func (inst *Transfer) SenderUserBank() *solana.AccountMeta {
	return inst.Accounts[1]
}

func (inst *Transfer) SetDestination(destination solana.PublicKey) *Transfer {
	inst.Accounts[2] = solana.Meta(destination).WRITE()
	return inst
}

func (inst *Transfer) Destination() *solana.AccountMeta {
	return inst.Accounts[2]
}

func (inst *Transfer) SetNonceAccount(nonce solana.PublicKey) *Transfer {
	inst.Accounts[3] = solana.Meta(nonce).WRITE()
	return inst
}

func (inst *Transfer) NonceAccount() *solana.AccountMeta {
	return inst.Accounts[3]
}

func (inst *Transfer) SetAuthority(authority solana.PublicKey) *Transfer {
	inst.Accounts[4] = solana.Meta(authority)
	return inst
}

func (inst *Transfer) Authority() *solana.AccountMeta {
	return inst.Accounts[4]
}

func (inst *Transfer) Validate() error {
	// Tests that the eth address is set to something non-zero
	if inst.SenderEthAddress.Big().Uint64() == 0 {
		return errors.New("senderEthAddress not set")
	}
	if inst.Payer() == nil {
		return errors.New("payer not set")
	}
	if inst.SenderUserBank() == nil {
		return errors.New("senderUserBank not set")
	}
	if inst.Destination() == nil {
		return errors.New("destination not set")
	}
	if inst.NonceAccount() == nil {
		return errors.New("nonce not set")
	}
	if inst.Authority() == nil {
		return errors.New("authority not set")
	}
	return nil
}

func (inst *Transfer) Build() *Instruction {
	return &Instruction{bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_Transfer),
	}}
}
func (inst Transfer) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *Transfer) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst *Transfer) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *Transfer) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("ClaimableTokens", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("Transfer")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("SenderEthAddress", inst.SenderEthAddress))
					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("Payer", inst.Payer().PublicKey))
						accountsBranch.Child(format.Account("SenderUserBank", inst.SenderUserBank().PublicKey))
						accountsBranch.Child(format.Account("Destination", inst.Destination().PublicKey))
						accountsBranch.Child(format.Account("Nonce", inst.NonceAccount().PublicKey))
						accountsBranch.Child(format.Account("Authority", inst.Authority().PublicKey))
					})
				})
		})
}

func NewTransferInstruction(
	senderEthAddress common.Address,
	mint solana.PublicKey,
	payer solana.PublicKey,
	destination solana.PublicKey,
) (*Transfer, error) {
	senderUserBank, err := deriveUserBankAccount(mint, senderEthAddress)
	if err != nil {
		return nil, err
	}
	authority, _, err := deriveAuthority(mint)
	if err != nil {
		return nil, err
	}
	nonce, _, err := deriveNonce(senderEthAddress, authority)
	if err != nil {
		return nil, err
	}
	return NewTransferInstructionBuilder().
			SetSenderEthAddress(senderEthAddress).
			SetPayer(payer).
			SetSenderUserBank(senderUserBank).
			SetDestination(destination).
			SetNonceAccount(nonce).
			SetAuthority(authority),
		nil
}
