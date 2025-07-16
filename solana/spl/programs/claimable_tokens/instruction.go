package claimable_tokens

import (
	"bytes"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/treeout"
)

const (
	NonceSeedPrefix      = "N_"
	EthAddressByteLength = 20
)

const (
	Instruction_CreateTokenAccount uint8 = iota
	Instruction_Transfer
)

// Represents a ClaimableTokens program instruction
type Instruction struct {
	bin.BaseVariant
}

// The current registered ProgramID for the ClaimableTokens instruction decoding
var ProgramID = solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ")

func init() {
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

// Changes the ProgramID used for decoding the ClaimableTokens instructions
func SetProgramID(pubkey solana.PublicKey) {
	ProgramID = pubkey
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

// Decodes a ClaimableTokens program instruction
func DecodeInstruction(accounts []*solana.AccountMeta, data []byte) (*Instruction, error) {
	inst := new(Instruction)
	if err := bin.NewBorshDecoder(data).Decode(inst); err != nil {
		return nil, fmt.Errorf("unable to decode instruction: %w", err)
	}
	if v, ok := inst.Impl.(solana.AccountsSettable); ok {
		err := v.SetAccounts(accounts)
		if err != nil {
			return nil, fmt.Errorf("unable to set accounts for instruction: %w", err)
		}
	}
	return inst, nil
}

// Implements solana.InstructionDecoder using DecodeInstruction
func registryDecodeInstruction(accounts []*solana.AccountMeta, data []byte) (interface{}, error) {
	inst, err := DecodeInstruction(accounts, data)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

var (
	_ solana.Instruction    = (*Instruction)(nil)
	_ text.TextEncodable    = (*Instruction)(nil)
	_ bin.BinaryUnmarshaler = (*Instruction)(nil)
	_ bin.BinaryMarshaler   = (*Instruction)(nil)
	_ text.EncodableToTree  = (*Instruction)(nil)
)

// ----- solana.Instruction Implementation -----

func (inst *Instruction) ProgramID() solana.PublicKey {
	return ProgramID
}

func (inst *Instruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

func (inst *Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// ----- text.TextEncodable Implementation -----

func (inst *Instruction) TextEncode(encoder *text.Encoder, option *text.Option) error {
	return encoder.Encode(inst.Impl, option)
}

// ----- text.EncodableToTree Implementation -----

func (inst *Instruction) EncodeToTree(parent treeout.Branches) {
	if enToTree, ok := inst.Impl.(text.EncodableToTree); ok {
		enToTree.EncodeToTree(parent)
	} else {
		parent.Child(spew.Sdump(inst))
	}
}

// ----- bin.BinaryUnmarshaler Implementation -----

var InstructionImplDef = bin.NewVariantDefinition(
	bin.Uint8TypeIDEncoding,
	[]bin.VariantType{
		{
			Name: "CreateTokenAccount", Type: (*CreateTokenAccount)(nil),
		},
		{
			Name: "Transfer", Type: (*Transfer)(nil),
		},
	},
)

func (inst *Instruction) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return inst.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionImplDef)
}

// ----- bin.BinaryMarshaler Implementation -----

func (inst Instruction) MarshalWithEncoder(encoder *bin.Encoder) error {
	err := encoder.WriteUint8(inst.TypeID.Uint8())
	if err != nil {
		return fmt.Errorf("unable to write variant type: %w", err)
	}
	return encoder.Encode(inst.Impl)
}
