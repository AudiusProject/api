package reward_manager

import (
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type SubmitAttestation struct {
	DisbursementID string

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewSubmitAttestationInstructionBuilder() *SubmitAttestation {
	inst := &SubmitAttestation{
		AccountMetaSlice: make(solana.AccountMetaSlice, 8),
	}
	inst.AccountMetaSlice[5] = solana.Meta(solana.SysVarRentPubkey)
	inst.AccountMetaSlice[6] = solana.Meta(solana.SysVarInstructionsPubkey)
	inst.AccountMetaSlice[7] = solana.Meta(solana.SystemProgramID)
	return inst
}

func (inst *SubmitAttestation) SetDisbursementId(disbursementId string) *SubmitAttestation {
	inst.DisbursementID = disbursementId
	return inst
}

func (inst *SubmitAttestation) SetAttestationsAccount(attestations solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[0] = solana.Meta(attestations).WRITE()
	return inst
}

func (inst *SubmitAttestation) GetAttestationsAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst *SubmitAttestation) SetRewardManagerStateAccount(rewardManagerState solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[1] = solana.Meta(rewardManagerState)
	return inst
}

func (inst *SubmitAttestation) GetRewardManagerStateAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

func (inst *SubmitAttestation) SetAuthorityAccount(authority solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[2] = solana.Meta(authority)
	return inst
}

func (inst *SubmitAttestation) GetAuthorityAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst *SubmitAttestation) SetPayerAccount(payer solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[3] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *SubmitAttestation) GetPayerAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst *SubmitAttestation) SetSenderAccount(sender solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[4] = solana.Meta(sender)
	return inst
}

func (inst *SubmitAttestation) GetSenderAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(4)
}

func (inst SubmitAttestation) Build() *Instruction {
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
	senderEthAddress common.Address,
	rewardManagerState solana.PublicKey,
	payer solana.PublicKey,
) (*SubmitAttestation, error) {
	disbursementId := challengeId + ":" + specifier
	authority, _, err := deriveAuthorityAccount(ProgramID, rewardManagerState)
	if err != nil {
		return nil, err
	}
	sender, _, err := deriveSenderAccount(ProgramID, authority, senderEthAddress)
	if err != nil {
		return nil, err
	}
	attestations, _, err := deriveAttestationsAccount(ProgramID, authority, disbursementId)
	if err != nil {
		return nil, err
	}

	return NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(disbursementId).
			SetPayerAccount(payer).
			SetAttestationsAccount(attestations).
			SetRewardManagerStateAccount(rewardManagerState).
			SetAuthorityAccount(authority).
			SetPayerAccount(payer).
			SetSenderAccount(sender),
		nil
}
