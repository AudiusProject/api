package reward_manager

import (
	"encoding/hex"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type EvaluateAttestation struct {
	// Instruction Data
	Amount               uint64
	DisbursementID       string
	ReceipientEthAddress string

	// Used for derivations
	RewardManagerState        solana.PublicKey `bin:"-" borsh_skip:"true"`
	Payer                     solana.PublicKey `bin:"-" borsh_skip:"true"`
	DestinationUserBank       solana.PublicKey `bin:"-" borsh_skip:"true"`
	TokenSource               solana.PublicKey `bin:"-" borsh_skip:"true"`
	AntiAbuseOracleEthAddress string           `bin:"-" borsh_skip:"true"`

	// Accounts
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

func NewEvaluateAttestationInstructionBuilder() *EvaluateAttestation {
	data := &EvaluateAttestation{}
	return data
}

func (inst *EvaluateAttestation) SetDisbursementID(challengedId string, specifier string) *EvaluateAttestation {
	inst.DisbursementID = challengedId + ":" + specifier
	return inst
}

func (inst *EvaluateAttestation) SetRecipientEthAddress(recipientEthAddress string) *EvaluateAttestation {
	inst.ReceipientEthAddress = recipientEthAddress
	return inst
}

func (inst *EvaluateAttestation) SetAmount(amount uint64) *EvaluateAttestation {
	inst.Amount = amount
	return inst
}

func (inst *EvaluateAttestation) SetAntiAbuseOracleEthAddress(antiAbuseOracleAddress string) *EvaluateAttestation {
	inst.AntiAbuseOracleEthAddress = antiAbuseOracleAddress
	return inst
}

func (inst *EvaluateAttestation) SetRewardManagerState(state solana.PublicKey) *EvaluateAttestation {
	inst.RewardManagerState = state
	return inst
}

func (inst *EvaluateAttestation) SetTokenSource(tokenSource solana.PublicKey) *EvaluateAttestation {
	inst.TokenSource = tokenSource
	return inst
}

func (inst *EvaluateAttestation) SetDestinationUserBank(userBank solana.PublicKey) *EvaluateAttestation {
	inst.DestinationUserBank = userBank
	return inst
}

func (inst *EvaluateAttestation) SetPayer(payer solana.PublicKey) *EvaluateAttestation {
	inst.Payer = payer
	return inst
}

func (inst EvaluateAttestation) Build() *Instruction {
	authority, _, _ := DeriveAuthorityAccount(ProgramID, inst.RewardManagerState)
	attestations, _, _ := DeriveAttestationsAccount(ProgramID, authority, inst.DisbursementID)
	disbursement, _, _ := deriveDisbursement(ProgramID, authority, inst.DisbursementID)
	antiAbuseOracle, _, _ := deriveSender(ProgramID, authority, inst.AntiAbuseOracleEthAddress)

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
			PublicKey:  inst.TokenSource,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  inst.DestinationUserBank,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  disbursement,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  antiAbuseOracle,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  inst.Payer,
			IsSigner:   true,
			IsWritable: true,
		},
		{
			PublicKey:  solana.SysVarRentPubkey,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.TokenProgramID,
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
		TypeID: bin.TypeIDFromUint8(Instruction_EvaluateAttestations),
	}}
}

func (inst EvaluateAttestation) MarshalWithEncoder(encoder *bin.Encoder) error {
	err := encoder.Encode(inst.Amount)
	if err != nil {
		return err
	}

	err = encoder.Encode(inst.DisbursementID)
	if err != nil {
		return err
	}

	address, err := hex.DecodeString(strings.TrimPrefix(inst.ReceipientEthAddress, "0x"))
	if err != nil {
		return err
	}
	return encoder.WriteBytes(address, false)
}

func (inst *EvaluateAttestation) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst)
}

func NewEvaluateAttestationInstruction(
	challengeId string,
	specifier string,
	recipientEthAddress string,
	amount uint64,
	antiAbuseOracleAddress string,
	rewardManagerState solana.PublicKey,
	tokenSource solana.PublicKey,
	destinationUserBank solana.PublicKey,
	payer solana.PublicKey,
) *EvaluateAttestation {
	return NewEvaluateAttestationInstructionBuilder().
		SetRewardManagerState(rewardManagerState).
		SetDisbursementID(challengeId, specifier).
		SetRecipientEthAddress(recipientEthAddress).
		SetAmount(amount).
		SetAntiAbuseOracleEthAddress(antiAbuseOracleAddress).
		SetTokenSource(tokenSource).
		SetDestinationUserBank(destinationUserBank).
		SetPayer(payer)
}
