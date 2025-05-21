package claimable_tokens

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type Transfer struct {
	SenderEthAddress common.Address
	Mint             solana.PublicKey `bin:"-" borsh_skip:"true"`
	Payer            solana.PublicKey `bin:"-" borsh_skip:"true"`
	Destination      solana.PublicKey `bin:"-" borsh_skip:"true"`

	// Accounts
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

type SignedTransfer struct {
	Destination solana.PublicKey
	Amount      uint64
	Nonce       uint64
}

func NewTransferInstructionBuilder() *Transfer {
	data := &Transfer{}
	return data
}

func (inst *Transfer) SetSenderEthAddress(ethAddress common.Address) *Transfer {
	inst.SenderEthAddress = ethAddress
	return inst
}

func (inst *Transfer) SetMint(mint solana.PublicKey) *Transfer {
	inst.Mint = mint
	return inst
}

func (inst *Transfer) SetPayer(payer solana.PublicKey) *Transfer {
	inst.Payer = payer
	return inst
}

func (inst *Transfer) SetDestination(destination solana.PublicKey) *Transfer {
	inst.Destination = destination
	return inst
}

func (inst Transfer) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *Transfer) Validate() error {
	if inst.SenderEthAddress.Big().Uint64() == 0 {
		return errors.New("senderEthAddress not set")
	}
	if inst.Mint.IsZero() {
		return errors.New("mint not set")
	}
	if inst.Payer.IsZero() {
		return errors.New("payer not set")
	}
	if inst.Destination.IsZero() {
		return errors.New("destination not set")
	}

	authority, _, err := deriveAuthority(inst.Mint)
	if err != nil {
		return err
	}
	_, err = deriveUserBankAccount(inst.Mint, inst.SenderEthAddress)
	if err != nil {
		return err
	}
	_, _, err = deriveNonce(inst.SenderEthAddress, authority)
	if err != nil {
		return err
	}
	return nil
}

func (inst Transfer) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteBytes(inst.SenderEthAddress.Bytes(), false)
}

func (inst *Transfer) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst)
}

func (inst *Transfer) Build() *Instruction {
	authority, _, _ := deriveAuthority(inst.Mint)
	sourceUserBank, _ := deriveUserBankAccount(inst.Mint, inst.SenderEthAddress)
	nonceAccount, _, _ := deriveNonce(inst.SenderEthAddress, authority)
	inst.AccountMetaSlice = []*solana.AccountMeta{
		{PublicKey: inst.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: sourceUserBank, IsSigner: false, IsWritable: true},
		{PublicKey: inst.Destination, IsSigner: false, IsWritable: true},
		{PublicKey: nonceAccount, IsSigner: false, IsWritable: true},
		{PublicKey: authority, IsSigner: false, IsWritable: false},
		{PublicKey: solana.SysVarRentPubkey, IsSigner: false, IsWritable: false},
		{PublicKey: solana.SysVarInstructionsPubkey, IsSigner: false, IsWritable: false},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		{PublicKey: solana.TokenProgramID, IsSigner: false, IsWritable: false},
	}
	return &Instruction{bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_Transfer),
	}}
}

func NewTransferInstruction(
	senderEthAddress common.Address,
	mint solana.PublicKey,
	payer solana.PublicKey,
	destination solana.PublicKey,
) *Transfer {
	return NewTransferInstructionBuilder().
		SetSenderEthAddress(senderEthAddress).
		SetMint(mint).
		SetPayer(payer).
		SetDestination(destination)
}
