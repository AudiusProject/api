package claimable_tokens

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type Transfer struct {
	// Instruction Data
	SenderEthAddress common.Address

	// Accounts
	Accounts solana.AccountMetaSlice
}

type SignedTransferData struct {
	Destination solana.PublicKey
	Amount      uint64
	Nonce       uint64
}

func NewTransferInstructionBuilder() *Transfer {
	data := &Transfer{
		Accounts: make(solana.AccountMetaSlice, 9),
	}
	data.Accounts[5] = solana.Meta(solana.SysVarRentPubkey)
	data.Accounts[6] = solana.Meta(solana.SysVarInstructionsPubkey)
	data.Accounts[7] = solana.Meta(solana.SystemProgramID)
	data.Accounts[8] = solana.Meta(solana.TokenProgramID)
	return data
}

func (inst *Transfer) SetSenderEthAddress(ethAddress common.Address) *Transfer {
	inst.SenderEthAddress = ethAddress
	return inst
}

func (inst *Transfer) SetPayer(payer solana.PublicKey) *Transfer {
	inst.Accounts[0] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *Transfer) GetPayer() *solana.AccountMeta {
	return inst.Accounts[0]
}

func (inst *Transfer) SetSenderUserBank(userBank solana.PublicKey) *Transfer {
	inst.Accounts[1] = solana.Meta(userBank).WRITE()
	return inst
}

func (inst *Transfer) GetSenderUserBank() *solana.AccountMeta {
	return inst.Accounts[1]
}

func (inst *Transfer) SetDestination(destination solana.PublicKey) *Transfer {
	inst.Accounts[2] = solana.Meta(destination).WRITE()
	return inst
}

func (inst *Transfer) GetDestination() *solana.AccountMeta {
	return inst.Accounts[2]
}

func (inst *Transfer) SetNonce(nonce solana.PublicKey) *Transfer {
	inst.Accounts[3] = solana.Meta(nonce).WRITE()
	return inst
}

func (inst *Transfer) GetNonce() *solana.AccountMeta {
	return inst.Accounts[3]
}

func (inst *Transfer) SetAuthority(authority solana.PublicKey) *Transfer {
	inst.Accounts[4] = solana.Meta(authority)
	return inst
}

func (inst *Transfer) GetAuthority() *solana.AccountMeta {
	return inst.Accounts[4]
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

func (inst *Transfer) Validate() error {
	// Tests that the eth address is set to something non-zero
	if inst.SenderEthAddress.Big().Uint64() == 0 {
		return errors.New("senderEthAddress not set")
	}
	if inst.GetPayer() == nil {
		return errors.New("payer not set")
	}
	if inst.GetSenderUserBank() == nil {
		return errors.New("senderUserBank not set")
	}
	if inst.GetDestination() == nil {
		return errors.New("destination not set")
	}
	if inst.GetNonce() == nil {
		return errors.New("nonce not set")
	}
	if inst.GetAuthority() == nil {
		return errors.New("authority not set")
	}
	return nil
}

func (inst *Transfer) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.Accounts = accounts
	return nil
}

func (inst Transfer) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteBytes(inst.SenderEthAddress.Bytes(), false)
}

func (inst *Transfer) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst.SenderEthAddress)
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
			SetNonce(nonce).
			SetAuthority(authority),
		nil
}
