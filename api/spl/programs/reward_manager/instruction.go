package reward_manager

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	ag_text "github.com/gagliardetto/solana-go/text"
)

const (
	SenderSeedPrefix       = "S_"
	EthAddressByteLength   = 20
	AttestationsSeedPrefix = "V_"
	DisbursementSeedPrefix = "T_"
)

var ProgramID = solana.MustPublicKeyFromBase58("CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp")

func SetProgramID(pubkey solana.PublicKey) {
	ProgramID = pubkey
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

func init() {
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

const (
	Instruction_Init uint8 = iota
	Instruction_ChangeManagerAccount
	Instruction_CreateSender
	Instruction_DeleteSender
	Instruction_CreateSenderPublic
	Instruction_DeleteSenderPublic
	Instruction_SubmitAttestation
	Instruction_EvaluateAttestations
)

type Instruction struct {
	bin.BaseVariant
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

func (inst *Instruction) TextEncode(encoder *ag_text.Encoder, option *ag_text.Option) error {
	return encoder.Encode(inst.Impl, option)
}

var InstructionImplDef = bin.NewVariantDefinition(
	bin.Uint8TypeIDEncoding,
	[]bin.VariantType{
		{
			Name: "Init", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "ChangeManagerAccount", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "CreateSender", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "DeleteSender", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "CreateSenderPublic", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "DeleteSenderPublic", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "SubmitAttestation", Type: (*SubmitAttestation)(nil),
		},
		{
			Name: "EvaluateAttestation", Type: (*EvaluateAttestation)(nil),
		},
	},
)

func (inst *Instruction) ProgramID() solana.PublicKey {
	return ProgramID
}

func (inst *Instruction) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return inst.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionImplDef)
}

func (inst Instruction) MarshalWithEncoder(encoder *bin.Encoder) error {
	err := encoder.WriteUint8(inst.TypeID.Uint8())
	if err != nil {
		return fmt.Errorf("unable to write variant type: %w", err)
	}
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

func deriveAuthority(programId solana.PublicKey, state solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := make([][]byte, 1)
	seeds[0] = state.Bytes()[0:32]
	return solana.FindProgramAddress(seeds, programId)
}

func deriveSender(programId solana.PublicKey, authority solana.PublicKey, ethAddress string) (solana.PublicKey, uint8, error) {
	senderSeedPrefix := []byte(SenderSeedPrefix)
	// Remove 0x and decode hex
	decodedEthAddress, err := hex.DecodeString(strings.TrimPrefix(ethAddress, "0x"))
	if err != nil {
		return solana.PublicKey{}, 0, err
	}
	// Pad the eth address if necessary w/ leading 0
	senderSeed := make([]byte, len(senderSeedPrefix)+EthAddressByteLength)
	copy(senderSeed, senderSeedPrefix)
	copy(senderSeed[len(senderSeed)-len(decodedEthAddress):], decodedEthAddress)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], senderSeed}, programId)
}

func deriveAttestations(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	attestationsSeed := make([]byte, len(AttestationsSeedPrefix)+len(disbursementId))
	copy(attestationsSeed, []byte(AttestationsSeedPrefix))
	copy(attestationsSeed[len([]byte(AttestationsSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], attestationsSeed}, programId)
}

func deriveDisbursement(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	disbursementSeed := make([]byte, len(DisbursementSeedPrefix)+len(disbursementId))
	copy(disbursementSeed, []byte(DisbursementSeedPrefix))
	copy(disbursementSeed[len([]byte(DisbursementSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], disbursementSeed}, programId)
}
