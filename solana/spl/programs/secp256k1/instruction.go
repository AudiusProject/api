package secp256k1

import (
	"bytes"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	ag_text "github.com/gagliardetto/solana-go/text"
)

type Instruction struct {
	bin.BaseVariant
}

func init() {
	solana.RegisterInstructionDecoder(solana.Secp256k1ProgramID, registryDecodeInstruction)
}

func (inst *Instruction) ProgramID() solana.PublicKey {
	return solana.Secp256k1ProgramID
}

func (inst *Instruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

func (inst *Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBinEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

func (inst *Instruction) TextEncode(encoder *ag_text.Encoder, option *ag_text.Option) error {
	return encoder.Encode(inst.Impl, option)
}

var InstructionImplDef = bin.NewVariantDefinition(
	bin.NoTypeIDEncoding,
	[]bin.VariantType{
		{
			Name: "Create", Type: (*Secp256k1Instruction)(nil),
		},
	},
)

func (inst *Instruction) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return inst.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionImplDef)
}

func (inst Instruction) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.Encode(inst.Impl)
}

func registryDecodeInstruction(accounts []*solana.AccountMeta, data []byte) (interface{}, error) {
	inst, err := DecodeInstruction(accounts, data)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func DecodeInstruction(accounts []*solana.AccountMeta, data []byte) (*Instruction, error) {
	inst := new(Instruction)
	if err := bin.NewBinDecoder(data).Decode(inst); err != nil {
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
