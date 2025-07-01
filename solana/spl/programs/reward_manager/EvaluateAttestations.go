package reward_manager

import (
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type EvaluateAttestation struct {
	Amount              uint64
	DisbursementId      string
	RecipientEthAddress common.Address

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewEvaluateAttestationInstructionBuilder() *EvaluateAttestation {
	inst := &EvaluateAttestation{
		AccountMetaSlice: make(solana.AccountMetaSlice, 11),
	}
	inst.AccountMetaSlice[8] = solana.Meta(solana.SysVarRentPubkey)
	inst.AccountMetaSlice[9] = solana.Meta(solana.TokenProgramID)
	inst.AccountMetaSlice[10] = solana.Meta(solana.SystemProgramID)
	return inst
}

func (inst *EvaluateAttestation) SetDisbursementId(disbursementId string) *EvaluateAttestation {
	inst.DisbursementId = disbursementId
	return inst
}

func (inst *EvaluateAttestation) SetRecipientEthAddress(recipientEthAddress common.Address) *EvaluateAttestation {
	inst.RecipientEthAddress = recipientEthAddress
	return inst
}

func (inst *EvaluateAttestation) SetAmount(amount uint64) *EvaluateAttestation {
	inst.Amount = amount
	return inst
}

func (inst *EvaluateAttestation) SetAttestationsAccount(state solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[0] = solana.Meta(state).WRITE()
	return inst
}

func (inst *EvaluateAttestation) GetAttestationsAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst *EvaluateAttestation) SetRewardManagerStateAccount(state solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[1] = solana.Meta(state)
	return inst
}

func (inst *EvaluateAttestation) GetRewardManagerStateAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

func (inst *EvaluateAttestation) SetAuthorityAccount(authority solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[2] = solana.Meta(authority)
	return inst
}

func (inst *EvaluateAttestation) GetAuthorityAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst *EvaluateAttestation) SetTokenSourceAccount(tokenSource solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[3] = solana.Meta(tokenSource).WRITE()
	return inst
}

func (inst *EvaluateAttestation) GetTokenSourceAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst *EvaluateAttestation) SetDestinationUserBankAccount(userBank solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[4] = solana.Meta(userBank).WRITE()
	return inst
}

func (inst *EvaluateAttestation) GetDestinationUserBankAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(4)
}

func (inst *EvaluateAttestation) SetDisbursementAccount(disbursement solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[5] = solana.Meta(disbursement).WRITE()
	return inst
}

func (inst *EvaluateAttestation) GetDisbursementAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(5)
}

func (inst *EvaluateAttestation) SetAntiAbuseOracleAccount(antiAbuseOracle solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[6] = solana.Meta(antiAbuseOracle)
	return inst
}

func (inst *EvaluateAttestation) GetAntiAbuseOracleAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(6)
}

func (inst *EvaluateAttestation) SetPayerAccount(payer solana.PublicKey) *EvaluateAttestation {
	inst.AccountMetaSlice[7] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *EvaluateAttestation) GetPayerAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice[7]
}

func (inst EvaluateAttestation) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_EvaluateAttestations),
	}}
}

func NewEvaluateAttestationInstruction(
	challengeId string,
	specifier string,
	recipientEthAddress common.Address,
	amount uint64,
	antiAbuseOracleAddress common.Address,
	rewardManagerState solana.PublicKey,
	tokenSource solana.PublicKey,
	destinationUserBank solana.PublicKey,
	payer solana.PublicKey,
) (*EvaluateAttestation, error) {
	disbursementId := challengeId + ":" + specifier
	authority, _, err := deriveAuthorityAccount(ProgramID, rewardManagerState)
	if err != nil {
		return nil, err
	}
	attestations, _, err := deriveAttestationsAccount(ProgramID, authority, disbursementId)
	if err != nil {
		return nil, err
	}
	disbursement, _, err := deriveDisbursementAccount(ProgramID, authority, disbursementId)
	if err != nil {
		return nil, err
	}
	antiAbuseOracle, _, err := deriveSenderAccount(ProgramID, authority, antiAbuseOracleAddress)
	if err != nil {
		return nil, err
	}

	return NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(disbursementId).
			SetRecipientEthAddress(recipientEthAddress).
			SetAttestationsAccount(attestations).
			SetRewardManagerStateAccount(rewardManagerState).
			SetAuthorityAccount(authority).
			SetTokenSourceAccount(tokenSource).
			SetDestinationUserBankAccount(destinationUserBank).
			SetDisbursementAccount(disbursement).
			SetAntiAbuseOracleAccount(antiAbuseOracle).
			SetPayerAccount(payer),
		nil
}
