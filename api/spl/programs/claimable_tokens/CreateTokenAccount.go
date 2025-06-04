package claimable_tokens

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type CreateTokenAccount struct {
	EthAddress common.Address
	Mint       solana.PublicKey `bin:"-" borsh_skip:"true"`
	Payer      solana.PublicKey `bin:"-" borsh_skip:"true"`

	// Accounts
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewCreateTokenAccountInstructionBuilder() *CreateTokenAccount {
	data := &CreateTokenAccount{}
	return data
}

func (inst *CreateTokenAccount) SetEthAddress(ethAddress common.Address) *CreateTokenAccount {
	inst.EthAddress = ethAddress
	return inst
}

func (inst *CreateTokenAccount) SetMint(mint solana.PublicKey) *CreateTokenAccount {
	inst.Mint = mint
	return inst
}

func (inst *CreateTokenAccount) SetPayer(payer solana.PublicKey) *CreateTokenAccount {
	inst.Payer = payer
	return inst
}

func (inst CreateTokenAccount) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *CreateTokenAccount) Validate() error {
	if inst.Payer.IsZero() {
		return errors.New("payer not set")
	}
	if inst.Mint.IsZero() {
		return errors.New("mint not set")
	}
	if inst.EthAddress.Big().Uint64() == uint64(0) {
		return errors.New("ethAddress not set")
	}

	_, _, err := deriveAuthority(inst.Mint)
	if err != nil {
		return err
	}

	_, err = deriveUserBankAccount(inst.Mint, inst.EthAddress)
	if err != nil {
		return err
	}
	return nil
}

func (inst *CreateTokenAccount) Build() *Instruction {
	authority, _, _ := deriveAuthority(inst.Mint)
	userBank, _ := deriveUserBankAccount(inst.Mint, inst.EthAddress)
	inst.AccountMetaSlice = []*solana.AccountMeta{
		{PublicKey: inst.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: inst.Mint, IsSigner: false, IsWritable: false},
		{PublicKey: authority, IsSigner: false, IsWritable: false},
		{PublicKey: userBank, IsSigner: false, IsWritable: true},
		{PublicKey: solana.SysVarRentPubkey, IsSigner: false, IsWritable: false},
		{PublicKey: solana.TokenProgramID, IsSigner: false, IsWritable: false},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
	}
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_CreateTokenAccount),
	}}
}

func (inst CreateTokenAccount) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteBytes(inst.EthAddress.Bytes(), false)
}

func (inst *CreateTokenAccount) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst)
}

func NewCreateTokenAccountInstruction(
	ethAddress common.Address,
	mint solana.PublicKey,
	payer solana.PublicKey,
) *CreateTokenAccount {
	return NewCreateTokenAccountInstructionBuilder().
		SetEthAddress(ethAddress).
		SetMint(mint).
		SetPayer(payer)
}
