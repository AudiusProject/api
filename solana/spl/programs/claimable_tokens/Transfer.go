package claimable_tokens

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type Transfer struct {
	SenderEthAddress common.Address
	SenderUserBank   solana.PublicKey `bin:"-" borsh_skip:"true"`
	Authority        solana.PublicKey `bin:"-" borsh_skip:"true"`
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

func (inst *Transfer) SetSender(ethAddress common.Address, mint solana.PublicKey) *Transfer {
	inst.SenderEthAddress = ethAddress
	userbank, _ := deriveUserBankAccount(mint, ethAddress)
	inst.SenderUserBank = userbank
	authority, _, _ := deriveAuthority(mint)
	inst.Authority = authority
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

func (inst *CreateTokenAccount) GetSenderUserBank() (solana.PublicKey, error) {
	builtUserBank := inst.AccountMetaSlice.Get(1)
	if builtUserBank != nil {
		return builtUserBank.PublicKey, nil
	}
	return deriveUserBankAccount(inst.Mint, inst.EthAddress)
}

func (inst Transfer) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *Transfer) Validate() error {
	if inst.SenderEthAddress.Big().Uint64() == 0 {
		return errors.New("sender not set")
	}
	if inst.Payer.IsZero() {
		return errors.New("payer not set")
	}
	if inst.Destination.IsZero() {
		return errors.New("destination not set")
	}
	if inst.SenderUserBank.IsZero() {
		return errors.New("sender has incorrect user bank")
	}
	if inst.Authority.IsZero() {
		return errors.New("sender has incorrect authority")
	}

	_, _, err := deriveNonce(inst.SenderEthAddress, inst.Authority)
	if err != nil {
		return err
	}
	return nil
}

func (inst *Transfer) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	payer := inst.AccountMetaSlice.Get(0)
	if payer != nil {
		inst.Payer = payer.PublicKey
	}
	senderUserBank := inst.AccountMetaSlice.Get(1)
	if senderUserBank != nil {
		inst.SenderUserBank = senderUserBank.PublicKey
	}
	destination := inst.AccountMetaSlice.Get(2)
	if destination != nil {
		inst.Destination = destination.PublicKey
	}
	authority := inst.AccountMetaSlice.Get(4)
	if authority != nil {
		inst.Authority = authority.PublicKey
	}
	return nil
}

func (inst Transfer) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteBytes(inst.SenderEthAddress.Bytes(), false)
}

func (inst *Transfer) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst.SenderEthAddress)
}

func (inst *Transfer) Build() *Instruction {
	nonceAccount, _, _ := deriveNonce(inst.SenderEthAddress, inst.Authority)
	inst.AccountMetaSlice = []*solana.AccountMeta{
		{PublicKey: inst.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: inst.SenderUserBank, IsSigner: false, IsWritable: true},
		{PublicKey: inst.Destination, IsSigner: false, IsWritable: true},
		{PublicKey: nonceAccount, IsSigner: false, IsWritable: true},
		{PublicKey: inst.Authority, IsSigner: false, IsWritable: false},
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
		SetSender(senderEthAddress, mint).
		SetPayer(payer).
		SetDestination(destination)
}
