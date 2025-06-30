package reward_manager

import (
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type EvaluateAttestation struct {
	// The instruction data
	Data EvaluateAttestationData

	// Exposed in decoded rpc responses

	// The account that paid the fees
	Payer solana.PublicKey
	// The destination user bank account
	DestinationUserBank solana.PublicKey

	// Used for derivations

	// The account holding the configuration state for the program
	rewardManagerState solana.PublicKey
	// The token account that holds the rewards
	tokenSource solana.PublicKey
	// The anti abuse oracle ethereum wallet address that has attested
	antiAbuseOracleEthAddress common.Address

	// Accounts
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

type EvaluateAttestationData struct {
	Amount               uint64
	DisbursementID       string
	ReceipientEthAddress common.Address
}

func NewEvaluateAttestationInstructionBuilder() *EvaluateAttestation {
	data := &EvaluateAttestation{}
	return data
}

func (inst *EvaluateAttestation) SetDisbursementID(challengedId string, specifier string) *EvaluateAttestation {
	inst.Data.DisbursementID = challengedId + ":" + specifier
	return inst
}

func (inst *EvaluateAttestation) SetRecipientEthAddress(recipientEthAddress common.Address) *EvaluateAttestation {
	inst.Data.ReceipientEthAddress = recipientEthAddress
	return inst
}

func (inst *EvaluateAttestation) SetAmount(amount uint64) *EvaluateAttestation {
	inst.Data.Amount = amount
	return inst
}

func (inst *EvaluateAttestation) SetAntiAbuseOracleEthAddress(antiAbuseOracleAddress common.Address) *EvaluateAttestation {
	inst.antiAbuseOracleEthAddress = antiAbuseOracleAddress
	return inst
}

func (inst *EvaluateAttestation) SetRewardManagerState(state solana.PublicKey) *EvaluateAttestation {
	inst.rewardManagerState = state
	return inst
}

func (inst *EvaluateAttestation) SetTokenSource(tokenSource solana.PublicKey) *EvaluateAttestation {
	inst.tokenSource = tokenSource
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
	authority, _, _ := deriveAuthorityAccount(ProgramID, inst.rewardManagerState)
	attestations, _, _ := deriveAttestationsAccount(ProgramID, authority, inst.Data.DisbursementID)
	disbursement, _, _ := deriveDisbursement(ProgramID, authority, inst.Data.DisbursementID)
	antiAbuseOracle, _, _ := deriveSender(ProgramID, authority, inst.antiAbuseOracleEthAddress)

	inst.AccountMetaSlice = []*solana.AccountMeta{
		{
			PublicKey:  attestations,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  inst.rewardManagerState,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  authority,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  inst.tokenSource,
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

func (inst *EvaluateAttestation) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	payer := inst.AccountMetaSlice.Get(7)
	if payer != nil {
		inst.Payer = payer.PublicKey
	}
	destinationUserBank := inst.AccountMetaSlice.Get(4)
	if destinationUserBank != nil {
		inst.DestinationUserBank = destinationUserBank.PublicKey
	}
	return nil
}

func (inst EvaluateAttestation) MarshalWithEncoder(encoder *bin.Encoder) error {
	err := encoder.Encode(inst.Data.Amount)
	if err != nil {
		return err
	}

	err = encoder.Encode(inst.Data.DisbursementID)
	if err != nil {
		return err
	}

	return encoder.WriteBytes(inst.Data.ReceipientEthAddress.Bytes(), false)
}

func (inst *EvaluateAttestation) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst.Data)
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
		SetRewardManagerState(rewardManagerState).
		SetDisbursementID(challengeId, specifier).
		SetRecipientEthAddress(recipientEthAddress).
		SetAmount(amount).
		SetAntiAbuseOracleEthAddress(antiAbuseOracleAddress).
		SetTokenSource(tokenSource).
		SetDestinationUserBank(destinationUserBank).
		SetPayer(payer)
}
