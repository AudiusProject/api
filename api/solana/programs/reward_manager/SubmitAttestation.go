package reward_manager

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type SubmitAttestation struct {
	// Instruction Data
	DisbursementID string

	// Used for derivations
	SenderEthAddress   string           `bin:"-" borsh_skip:"true"`
	RewardManagerState solana.PublicKey `bin:"-" borsh_skip:"true"`
	Payer              solana.PublicKey `bin:"-" borsh_skip:"true"`

	// Accounts
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
