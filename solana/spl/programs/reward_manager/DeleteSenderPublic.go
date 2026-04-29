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

// DeleteSenderPublic mirrors CreateSenderPublic but for removing a sender
// from the rewards-manager program. Authorization comes from secp256k1
// instructions earlier in the same transaction (one per attester),
// each signing over "del" + rewardManagerState(32) + ethAddress(20).
//
// Account layout (matches Rust `delete_sender_public` in
// apps/solana-programs/reward-manager/program/src/instruction.rs):
//
//	0. []         reward_manager state
//	1. [writable] sender PDA being deleted
//	2. [writable] refunder (rent recipient)
//	3. []         sysvar instructions
//	4..n. []      attester sender PDAs
type DeleteSenderPublic struct {
	// EthAddress is used to derive the sender PDA but is NOT serialized into
	// the instruction data — the on-chain Instructions::DeleteSenderPublic
	// variant carries no fields. The program reads the target sender's eth
	// address from the sender PDA itself, not from instruction data.
	EthAddress common.Address `bin:"-" borsh_skip:"true"`

	Attesters []common.Address `bin:"-" borsh_skip:"true"`

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*DeleteSenderPublic)(nil)
	_ solana.AccountsSettable = (*DeleteSenderPublic)(nil)
	_ text.EncodableToTree    = (*DeleteSenderPublic)(nil)
)

func NewDeleteSenderPublicInstructionBuilder() *DeleteSenderPublic {
	inst := &DeleteSenderPublic{
		Accounts: make(solana.AccountMetaSlice, 4),
	}
	inst.Accounts[3] = solana.Meta(solana.SysVarInstructionsPubkey)
	return inst
}

func (inst *DeleteSenderPublic) SetEthAddress(ethAddress common.Address) *DeleteSenderPublic {
	inst.EthAddress = ethAddress
	return inst
}

func (inst *DeleteSenderPublic) SetRewardManagerStateAccount(state solana.PublicKey) *DeleteSenderPublic {
	inst.Accounts[0] = solana.Meta(state)
	return inst
}

func (inst *DeleteSenderPublic) RewardManagerStateAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *DeleteSenderPublic) SetSenderAccount(sender solana.PublicKey) *DeleteSenderPublic {
	inst.Accounts[1] = solana.Meta(sender).WRITE()
	return inst
}

func (inst *DeleteSenderPublic) SenderAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *DeleteSenderPublic) SetRefunderAccount(refunder solana.PublicKey) *DeleteSenderPublic {
	inst.Accounts[2] = solana.Meta(refunder).WRITE()
	return inst
}

func (inst *DeleteSenderPublic) RefunderAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *DeleteSenderPublic) AddAttester(attester solana.PublicKey) *DeleteSenderPublic {
	inst.Accounts = append(inst.Accounts, solana.Meta(attester))
	return inst
}

func (inst *DeleteSenderPublic) Validate() error {
	if inst.EthAddress == (common.Address{}) {
		return errors.New("ethAddress not set")
	}
	if inst.RewardManagerStateAccount() == nil {
		return errors.New("rewardManagerState account not set")
	}
	if inst.RefunderAccount() == nil {
		return errors.New("refunder account not set")
	}

	authority, _, err := deriveAuthorityAccount(ProgramID, inst.RewardManagerStateAccount().PublicKey)
	if err != nil {
		return fmt.Errorf("failed to derive authority account: %w", err)
	}

	_, _, err = DeriveSenderAccount(ProgramID, authority, inst.EthAddress)
	if err != nil {
		return fmt.Errorf("failed to derive sender account: %w", err)
	}

	for _, addr := range inst.Attesters {
		_, _, err = DeriveSenderAccount(ProgramID, authority, addr)
		if err != nil {
			return fmt.Errorf("failed to derive sender account for attester %s: %w", addr.Hex(), err)
		}
	}

	return nil
}

// Build builds the instruction
func (inst DeleteSenderPublic) Build() *Instruction {
	authority, _, _ := deriveAuthorityAccount(ProgramID, inst.RewardManagerStateAccount().PublicKey)
	sender, _, _ := DeriveSenderAccount(ProgramID, authority, inst.EthAddress)
	inst.SetSenderAccount(sender)

	for _, addr := range inst.Attesters {
		attesterSender, _, _ := DeriveSenderAccount(ProgramID, authority, addr)
		inst.AddAttester(attesterSender)
	}

	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_DeleteSenderPublic),
	}}
}

// ValidateAndBuild validates and builds the instruction
func (inst *DeleteSenderPublic) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *DeleteSenderPublic) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst DeleteSenderPublic) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *DeleteSenderPublic) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("DeleteSenderPublic", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Param("EthAddress", inst.EthAddress.Hex()))
			programBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
				if inst.RewardManagerStateAccount() != nil {
					accountsBranch.Child(format.Meta("rewardManagerState", inst.RewardManagerStateAccount()))
				}
				if inst.SenderAccount() != nil {
					accountsBranch.Child(format.Meta("sender", inst.SenderAccount()))
				}
				if inst.RefunderAccount() != nil {
					accountsBranch.Child(format.Meta("refunder", inst.RefunderAccount()))
				}
				for i, acct := range inst.Accounts[4:] {
					accountsBranch.Child(format.Meta(
						"attester_"+strconv.Itoa(i),
						acct,
					))
				}
			})
		})
}

// NewDeleteSenderPublicInstruction creates a new DeleteSenderPublic instruction.
// `ethAddress` is the sender being removed; `attesterEthAddresses` are the
// existing senders signing the delete attestation.
func NewDeleteSenderPublicInstruction(
	ethAddress common.Address,
	rewardManagerState solana.PublicKey,
	refunder solana.PublicKey,
	attesterEthAddresses ...common.Address,
) (*DeleteSenderPublic, error) {
	inst := NewDeleteSenderPublicInstructionBuilder().
		SetEthAddress(ethAddress).
		SetRewardManagerStateAccount(rewardManagerState).
		SetRefunderAccount(refunder)

	inst.Attesters = append(inst.Attesters, attesterEthAddresses...)

	return inst, nil
}
