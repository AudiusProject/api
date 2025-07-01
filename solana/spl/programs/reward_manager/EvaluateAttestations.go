package reward_manager

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type EvaluateAttestation struct {
	Amount              *uint64
	DisbursementId      *string
	RecipientEthAddress *common.Address

	// Optional: Only used when constructing a new instruction
	antiAbuseOracleEthAddress *common.Address

	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewEvaluateAttestationInstructionBuilder() *EvaluateAttestation {
	data := &EvaluateAttestation{
		AccountMetaSlice: make(solana.AccountMetaSlice, 11),
	}
	data.AccountMetaSlice[8] = solana.Meta(solana.SysVarRentPubkey)
	data.AccountMetaSlice[9] = solana.Meta(solana.TokenProgramID)
	data.AccountMetaSlice[10] = solana.Meta(solana.SystemProgramID)
	return data
}

func (inst *EvaluateAttestation) SetDisbursementId(disbursementId string) *EvaluateAttestation {
	inst.DisbursementId = &disbursementId
	return inst
}

func (inst *EvaluateAttestation) SetRecipientEthAddress(recipientEthAddress common.Address) *EvaluateAttestation {
	inst.RecipientEthAddress = &recipientEthAddress
	return inst
}

func (inst *EvaluateAttestation) SetAmount(amount uint64) *EvaluateAttestation {
	inst.Amount = &amount
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

func (inst *EvaluateAttestation) SetAntiAbuseOracleEthAddress(antiAbuseOracle common.Address) *EvaluateAttestation {
	inst.antiAbuseOracleEthAddress = &antiAbuseOracle
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

func (inst *EvaluateAttestation) Validate() error {
	if inst.Amount == nil {
		return errors.New("amount is not set")
	}

	if inst.DisbursementId == nil {
		return errors.New("disbursementId is not set")
	}

	if inst.RecipientEthAddress == nil {
		return errors.New("recipientEthAddress is not set")
	}

	if inst.GetRewardManagerStateAccount() == nil {
		return errors.New("rewardManagerStateAccount is not set")
	}

	if inst.GetTokenSourceAccount() == nil {
		return errors.New("tokenSourceAccount is not set")
	}

	if inst.GetPayerAccount() == nil {
		return errors.New("payerAccount is not set")
	}

	authority, _, err := deriveAuthorityAccount(ProgramID, inst.GetRewardManagerStateAccount().PublicKey)
	if err != nil {
		return err
	}
	if inst.GetAuthorityAccount() == nil {
		inst.SetAuthorityAccount(authority)
	} else if !inst.GetAuthorityAccount().PublicKey.Equals(authority) {
		return errors.New("manually set authority does not match derived authority")
	}

	attestations, _, err := deriveAttestationsAccount(ProgramID, authority, *inst.DisbursementId)
	if err != nil {
		return err
	}
	if inst.GetAttestationsAccount() == nil {
		inst.SetAttestationsAccount(attestations)
	} else if !inst.GetAttestationsAccount().PublicKey.Equals(attestations) {
		return errors.New("manually set attestations account does not match derived attestations account")
	}

	disbursements, _, err := deriveDisbursementAccount(ProgramID, authority, *inst.DisbursementId)
	if err != nil {
		return err
	}
	if inst.GetDisbursementAccount() == nil {
		inst.SetDisbursementAccount(disbursements)
	} else if !inst.GetDisbursementAccount().PublicKey.Equals(disbursements) {
		return errors.New("manually set disbursement account does not match derived disbursments account")
	}

	if inst.antiAbuseOracleEthAddress != nil {
		antiAbuseOracle, _, err := deriveSenderAccount(ProgramID, authority, *inst.antiAbuseOracleEthAddress)
		if err != nil {
			return err
		}
		if inst.GetAntiAbuseOracleAccount() == nil {
			inst.SetAntiAbuseOracleAccount(antiAbuseOracle)
		}
	} else if inst.GetAntiAbuseOracleAccount() == nil {
		return errors.New("neither antiAbuseOracle nor antiAbuseOracleEthAddress are set")
	}

	return nil
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
) *EvaluateAttestation {
	return NewEvaluateAttestationInstructionBuilder().
		SetAmount(amount).
		SetDisbursementId(challengeId + ":" + specifier).
		SetRecipientEthAddress(recipientEthAddress).
		SetRewardManagerStateAccount(rewardManagerState).
		SetTokenSourceAccount(tokenSource).
		SetDestinationUserBankAccount(destinationUserBank).
		SetPayerAccount(payer).
		SetAntiAbuseOracleEthAddress(antiAbuseOracleAddress)
}
