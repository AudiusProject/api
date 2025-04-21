package reward_manager

import (
	"encoding/hex"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

const (
	SenderSeedPrefix       = "S_"
	EthAddressByteLength   = 20
	AttestationsSeedPrefix = "V_"
)

type SubmitAttestation struct {
	DisbursementID     string
	SenderEthAddress   string           `bin:"-" borsh_skip:"true"`
	RewardManagerState solana.PublicKey `bin:"-" borsh_skip:"true"`
	Payer              solana.PublicKey `bin:"-" borsh_skip:"true"`

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewSubmitAttestationInstructionBuilder() *SubmitAttestation {
	nd := &SubmitAttestation{}
	return nd
}

func (inst *SubmitAttestation) SetDisbursementID(challengeId string, specifier string) *SubmitAttestation {
	inst.DisbursementID = challengeId + ":" + specifier
	return inst
}

func (inst *SubmitAttestation) SetSenderEthAddress(senderEthAddress string) *SubmitAttestation {
	inst.SenderEthAddress = senderEthAddress
	return inst
}

func (inst *SubmitAttestation) SetRewardManagerState(state solana.PublicKey) *SubmitAttestation {
	inst.RewardManagerState = state
	return inst
}

func (inst *SubmitAttestation) SetPayer(payer solana.PublicKey) *SubmitAttestation {
	inst.Payer = payer
	return inst
}

func deriveAuthority(programId solana.PublicKey, state solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := make([][]byte, 1)
	seeds[0] = state.Bytes()[0:32]
	return solana.FindProgramAddress(seeds, programId)
}

func deriveSender(programId solana.PublicKey, authority solana.PublicKey, ethAddress string) (solana.PublicKey, uint8, error) {

	senderSeedPrefix := []byte(SenderSeedPrefix)
	// Remove 0x and decode hex
	decodedEthAddress, err := hex.DecodeString(ethAddress[2:])
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

func (inst SubmitAttestation) Build() *Instruction {
	authority, _, _ := deriveAuthority(ProgramID, inst.RewardManagerState)
	sender, _, _ := deriveSender(ProgramID, authority, inst.SenderEthAddress)
	attestations, _, _ := deriveAttestations(ProgramID, authority, inst.DisbursementID)

	inst.AccountMetaSlice = []*solana.AccountMeta{
		{
			PublicKey:  attestations,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  inst.RewardManagerState,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  authority,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  inst.Payer,
			IsSigner:   true,
			IsWritable: false,
		},
		{
			PublicKey:  sender,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SysVarRentPubkey,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SysVarInstructionsPubkey,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SystemProgramID,
			IsSigner:   false,
			IsWritable: false,
		},
	}

	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_SubmitAttestation),
	}}
}

func (inst SubmitAttestation) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.WriteString(inst.DisbursementID)
}

func (inst *SubmitAttestation) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst.DisbursementID)
}

func NewSubmitAttestationInstruction(
	challengeId string,
	specifier string,
	senderEthAddress string,
	rewardManagerState solana.PublicKey,
	payer solana.PublicKey,

) *SubmitAttestation {
	return NewSubmitAttestationInstructionBuilder().
		SetRewardManagerState(rewardManagerState).
		SetDisbursementID(challengeId, specifier).
		SetSenderEthAddress(senderEthAddress).
		SetPayer(payer)
}
