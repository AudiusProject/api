package reward_manager

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type CreateSenderPublic struct {
	EthAddress         common.Address
	OperatorEthAddress common.Address

	Attesters []common.Address `bin:"-" borsh_skip:"true"`

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*CreateSenderPublic)(nil)
	_ solana.AccountsSettable = (*CreateSenderPublic)(nil)
	_ text.EncodableToTree    = (*CreateSenderPublic)(nil)
)

func NewCreateSenderPublicInstructionBuilder() *CreateSenderPublic {
	inst := &CreateSenderPublic{
		Accounts: make(solana.AccountMetaSlice, 7),
	}
	inst.Accounts[4] = solana.Meta(solana.SysVarInstructionsPubkey)
	inst.Accounts[5] = solana.Meta(solana.SysVarRentPubkey)
	inst.Accounts[6] = solana.Meta(solana.SystemProgramID)
	return inst
}

func (inst *CreateSenderPublic) SetEthAddress(ethAddress common.Address) *CreateSenderPublic {
	inst.EthAddress = ethAddress
	return inst
}

func (inst *CreateSenderPublic) SetOperatorEthAddress(operatorEthAddress common.Address) *CreateSenderPublic {
	inst.OperatorEthAddress = operatorEthAddress
	return inst
}

func (inst *CreateSenderPublic) SetRewardManagerStateAccount(state solana.PublicKey) *CreateSenderPublic {
	inst.Accounts[0] = solana.Meta(state)
	return inst
}

func (inst *CreateSenderPublic) RewardManagerStateAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *CreateSenderPublic) SetAuthorityAccount(authority solana.PublicKey) *CreateSenderPublic {
	inst.Accounts[1] = solana.Meta(authority)
	return inst
}

func (inst *CreateSenderPublic) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *CreateSenderPublic) SetPayerAccount(payer solana.PublicKey) *CreateSenderPublic {
	inst.Accounts[2] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *CreateSenderPublic) PayerAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *CreateSenderPublic) SetSenderAccount(sender solana.PublicKey) *CreateSenderPublic {
	inst.Accounts[3] = solana.Meta(sender).WRITE()
	return inst
}

func (inst *CreateSenderPublic) SenderAccount() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *CreateSenderPublic) AddAttester(attester solana.PublicKey) *CreateSenderPublic {
	inst.Accounts = append(inst.Accounts, solana.Meta(attester))
	return inst
}

func (inst *CreateSenderPublic) Validate() error {
	if inst.EthAddress == (common.Address{}) {
		return errors.New("ethAddress not set")
	}
	if inst.OperatorEthAddress == (common.Address{}) {
		return errors.New("operatorEthAddress not set")
	}
	if inst.SenderAccount() == nil {
		return errors.New("sender account not set")
	}
	if inst.RewardManagerStateAccount() == nil {
		return errors.New("rewardManagerState account not set")
	}
	if inst.PayerAccount() == nil {
		return errors.New("payer account not set")
	}

	_, _, err := deriveAuthorityAccount(ProgramID, inst.RewardManagerStateAccount().PublicKey)
	if err != nil {
		return fmt.Errorf("failed to derive authority account: %w", err)
	}

	_, _, err = DeriveSenderAccount(ProgramID, inst.AuthorityAccount().PublicKey, inst.EthAddress)
	if err != nil {
		return fmt.Errorf("failed to derive sender account: %w", err)
	}

	for _, addr := range inst.Attesters {
		_, _, err = DeriveSenderAccount(ProgramID, inst.AuthorityAccount().PublicKey, addr)
		if err != nil {
			return fmt.Errorf("failed to derive sender account for attester %s: %w", addr.Hex(), err)
		}
	}

	return nil
}

// Build builds the instruction
func (inst CreateSenderPublic) Build() *Instruction {

	authority, _, _ := deriveAuthorityAccount(ProgramID, inst.RewardManagerStateAccount().PublicKey)
	inst.SetAuthorityAccount(authority)
	newSender, _, _ := DeriveSenderAccount(ProgramID, authority, inst.EthAddress)
	inst.SetSenderAccount(newSender)

	for _, addr := range inst.Attesters {
		sender, _, _ := DeriveSenderAccount(ProgramID, authority, addr)
		inst.AddAttester(sender)
	}

	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_CreateSenderPublic),
	}}
}

// ValidateAndBuild validates and builds the instruction
func (inst *CreateSenderPublic) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *CreateSenderPublic) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst CreateSenderPublic) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *CreateSenderPublic) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("CreateSenderPublic", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Param("EthAddress", inst.EthAddress.Hex()))
			programBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
				if inst.SenderAccount() != nil {
					accountsBranch.Child(format.Meta("sender", inst.SenderAccount()))
				}
				if inst.RewardManagerStateAccount() != nil {
					accountsBranch.Child(format.Meta("rewardManagerState", inst.RewardManagerStateAccount()))
				}
				if inst.AuthorityAccount() != nil {
					accountsBranch.Child(format.Meta("authority", inst.AuthorityAccount()))
				}
				if inst.PayerAccount() != nil {
					accountsBranch.Child(format.Meta("payer", inst.PayerAccount()))
				}
				for i, acct := range inst.Accounts[7:] {
					accountsBranch.Child(format.Meta(
						"attester_"+strconv.Itoa(i),
						acct,
					))
				}
			})
		})
}

// NewCreateSenderPublicInstruction creates a new CreateSenderPublic instruction
func NewCreateSenderPublicInstruction(
	ethAddress common.Address,
	operatorEthAddress common.Address,
	rewardManagerState solana.PublicKey,
	payer solana.PublicKey,
	attesterEthAddresses ...common.Address,
) (*CreateSenderPublic, error) {

	inst := NewCreateSenderPublicInstructionBuilder().
		SetEthAddress(ethAddress).
		SetOperatorEthAddress(operatorEthAddress).
		SetRewardManagerStateAccount(rewardManagerState).
		SetPayerAccount(payer)

	inst.Attesters = append(inst.Attesters, attesterEthAddresses...)

	return inst, nil
}
