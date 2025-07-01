package reward_manager

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type SubmitAttestation struct {
	DisbursementId string

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*SubmitAttestation)(nil)
	_ solana.AccountsSettable = (*SubmitAttestation)(nil)
	_ text.EncodableToTree    = (*SubmitAttestation)(nil)
)

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
	inst.DisbursementId = disbursementId
	return inst
}

func (inst *SubmitAttestation) SetAttestationsAccount(attestations solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[0] = solana.Meta(attestations).WRITE()
	return inst
}

func (inst *SubmitAttestation) AttestationsAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst *SubmitAttestation) SetRewardManagerStateAccount(rewardManagerState solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[1] = solana.Meta(rewardManagerState)
	return inst
}

func (inst *SubmitAttestation) RewardManagerStateAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

func (inst *SubmitAttestation) SetAuthorityAccount(authority solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[2] = solana.Meta(authority)
	return inst
}

func (inst *SubmitAttestation) AuthorityAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst *SubmitAttestation) SetPayerAccount(payer solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[3] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *SubmitAttestation) PayerAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst *SubmitAttestation) SetSenderAccount(sender solana.PublicKey) *SubmitAttestation {
	inst.AccountMetaSlice[4] = solana.Meta(sender)
	return inst
}

func (inst *SubmitAttestation) SenderAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(4)
}

func (inst *SubmitAttestation) Validate() error {
	if inst.DisbursementId == "" {
		return errors.New("disbursementId not set")
	}
	if inst.AttestationsAccount() == nil {
		return errors.New("attestations account not set")
	}
	if inst.RewardManagerStateAccount() == nil {
		return errors.New("rewardManagerState account not set")
	}
	if inst.AuthorityAccount() == nil {
		return errors.New("authority account not set")
	}
	if inst.PayerAccount() == nil {
		return errors.New("payer account not set")
	}
	if inst.SenderAccount() == nil {
		return errors.New("sender account not set")
	}
	return nil
}

func (inst SubmitAttestation) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_SubmitAttestation),
	}}
}

func (inst *SubmitAttestation) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *SubmitAttestation) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("RewardManager", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("EvaluateAttestations")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("DisbursementId", inst.DisbursementId))

					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("Attestations", inst.AttestationsAccount().PublicKey))
						accountsBranch.Child(format.Account("State", inst.RewardManagerStateAccount().PublicKey))
						accountsBranch.Child(format.Account("Authority", inst.AuthorityAccount().PublicKey))
						accountsBranch.Child(format.Account("Payer", inst.PayerAccount().PublicKey))
						accountsBranch.Child(format.Account("Sender", inst.SenderAccount().PublicKey))
					})
				})
		})
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
