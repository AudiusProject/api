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

type EvaluateAttestation struct {
	Amount              uint64
	DisbursementId      string
	RecipientEthAddress common.Address

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*EvaluateAttestation)(nil)
	_ solana.AccountsSettable = (*EvaluateAttestation)(nil)
	_ text.EncodableToTree    = (*EvaluateAttestation)(nil)
)

func NewEvaluateAttestationInstructionBuilder() *EvaluateAttestation {
	inst := &EvaluateAttestation{
		Accounts: make(solana.AccountMetaSlice, 11),
	}
	inst.Accounts[8] = solana.Meta(solana.SysVarRentPubkey)
	inst.Accounts[9] = solana.Meta(solana.TokenProgramID)
	inst.Accounts[10] = solana.Meta(solana.SystemProgramID)
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
	inst.Accounts[0] = solana.Meta(state).WRITE()
	return inst
}

func (inst *EvaluateAttestation) AttestationsAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *EvaluateAttestation) SetRewardManagerStateAccount(state solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[1] = solana.Meta(state)
	return inst
}

func (inst *EvaluateAttestation) RewardManagerStateAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *EvaluateAttestation) SetAuthorityAccount(authority solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[2] = solana.Meta(authority)
	return inst
}

func (inst *EvaluateAttestation) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *EvaluateAttestation) SetTokenSourceAccount(tokenSource solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[3] = solana.Meta(tokenSource).WRITE()
	return inst
}

func (inst *EvaluateAttestation) TokenSourceAccount() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *EvaluateAttestation) SetDestinationUserBankAccount(userBank solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[4] = solana.Meta(userBank).WRITE()
	return inst
}

func (inst *EvaluateAttestation) DestinationUserBankAccount() *solana.AccountMeta {
	return inst.Accounts.Get(4)
}

func (inst *EvaluateAttestation) SetDisbursementAccount(disbursement solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[5] = solana.Meta(disbursement).WRITE()
	return inst
}

func (inst *EvaluateAttestation) DisbursementAccount() *solana.AccountMeta {
	return inst.Accounts.Get(5)
}

func (inst *EvaluateAttestation) SetAntiAbuseOracleAccount(antiAbuseOracle solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[6] = solana.Meta(antiAbuseOracle)
	return inst
}

func (inst *EvaluateAttestation) AntiAbuseOracleAccount() *solana.AccountMeta {
	return inst.Accounts.Get(6)
}

func (inst *EvaluateAttestation) SetPayerAccount(payer solana.PublicKey) *EvaluateAttestation {
	inst.Accounts[7] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *EvaluateAttestation) PayerAccount() *solana.AccountMeta {
	return inst.Accounts[7]
}

func (inst *EvaluateAttestation) Validate() error {
	if inst.DisbursementId == "" {
		return errors.New("disbursementId not set")
	}
	if inst.RecipientEthAddress.Big().Uint64() == 0 {
		return errors.New("recipientEthAddress not set")
	}
	if inst.Amount == 0 {
		return errors.New("amount not set")
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
	if inst.TokenSourceAccount() == nil {
		return errors.New("tokenSource account not set")
	}
	if inst.DestinationUserBankAccount() == nil {
		return errors.New("destinationUserBank account not set")
	}
	if inst.DisbursementAccount() == nil {
		return errors.New("disbursement account not set")
	}
	if inst.AntiAbuseOracleAccount() == nil {
		return errors.New("antiAbuseOracle account not set")
	}
	if inst.PayerAccount() == nil {
		return errors.New("payer account not set")
	}
	return nil
}

func (inst EvaluateAttestation) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_EvaluateAttestations),
	}}
}

func (inst EvaluateAttestation) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *EvaluateAttestation) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst *EvaluateAttestation) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *EvaluateAttestation) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("RewardManager", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("EvaluateAttestations")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("Amount", inst.Amount))
						paramsBranch.Child(format.Param("DisbursementId", inst.DisbursementId))
						paramsBranch.Child(format.Param("RecipientEthAddress", inst.RecipientEthAddress))

					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("Attestations", inst.AttestationsAccount().PublicKey))
						accountsBranch.Child(format.Account("State", inst.RewardManagerStateAccount().PublicKey))
						accountsBranch.Child(format.Account("Authority", inst.AuthorityAccount().PublicKey))
						accountsBranch.Child(format.Account("TokenSource", inst.TokenSourceAccount().PublicKey))
						accountsBranch.Child(format.Account("Destination", inst.DestinationUserBankAccount().PublicKey))
						accountsBranch.Child(format.Account("Disbursement", inst.DisbursementAccount().PublicKey))
						accountsBranch.Child(format.Account("AntiAbuseOracle", inst.AntiAbuseOracleAccount().PublicKey))
						accountsBranch.Child(format.Account("Payer", inst.PayerAccount().PublicKey))
					})
				})
		})
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
