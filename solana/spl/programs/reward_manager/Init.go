package reward_manager

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type Init struct {
	MinVotes uint8

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*Init)(nil)
	_ solana.AccountsSettable = (*Init)(nil)
	_ text.EncodableToTree    = (*Init)(nil)
)

func NewInitInstructionBuilder() *Init {
	inst := &Init{
		Accounts: make(solana.AccountMetaSlice, 6),
	}
	inst.Accounts[5] = solana.Meta(solana.TokenProgramID)
	inst.Accounts[6] = solana.Meta(solana.SysVarRentPubkey)
	return inst
}

func (inst *Init) SetMinVotes(minVotes uint8) *Init {
	inst.MinVotes = minVotes
	return inst
}

func (inst *Init) SetRewardManagerState(state solana.PublicKey) *Init {
	inst.Accounts[0] = solana.Meta(state).WRITE()
	return inst
}

func (inst *Init) RewardManagerState() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *Init) SetTokenSourceAccount(tokenSource solana.PublicKey) *Init {
	inst.Accounts[1] = solana.Meta(tokenSource).WRITE()
	return inst
}

func (inst *Init) TokenSourceAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *Init) SetMint(mint solana.PublicKey) *Init {
	inst.Accounts[2] = solana.Meta(mint)
	return inst
}

func (inst *Init) Mint() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *Init) SetManager(manager solana.PublicKey) *Init {
	inst.Accounts[3] = solana.Meta(manager)
	return inst
}

func (inst *Init) Manager() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *Init) SetAuthority(authority solana.PublicKey) *Init {
	inst.Accounts[4] = solana.Meta(authority)
	return inst
}

func (inst *Init) Authority() *solana.AccountMeta {
	return inst.Accounts.Get(4)
}

func (inst *Init) Validate() error {
	if inst.MinVotes == 0 {
		return fmt.Errorf("MinVotes must be greater than zero")
	}
	if inst.RewardManagerState() == nil {
		return fmt.Errorf("missing RewardManagerState account")
	}
	if inst.TokenSourceAccount() == nil {
		return fmt.Errorf("missing TokenSource account")
	}
	if inst.Mint() == nil {
		return fmt.Errorf("missing Mint account")
	}
	if inst.Manager() == nil {
		return fmt.Errorf("missing Manager account")
	}
	if inst.Authority() == nil {
		return fmt.Errorf("missing Authority account")
	}
	return nil
}

func (inst Init) Build() *Instruction {
	return &Instruction{bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_Init),
	}}
}

func (inst Init) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// ----- solana.AccountsSettable Implementation -----

func (inst *Init) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst Init) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst Init) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("RewardManager", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("Init")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child(format.Param("MinVotes", inst.MinVotes))
					instructionBranch.Child(format.Account("RewardManagerState", inst.RewardManagerState().PublicKey))
					instructionBranch.Child(format.Account("TokenSource", inst.TokenSourceAccount().PublicKey))
					instructionBranch.Child(format.Account("Mint", inst.Mint().PublicKey))
					instructionBranch.Child(format.Account("Manager", inst.Manager().PublicKey))
					instructionBranch.Child(format.Account("Authority", inst.Authority().PublicKey))
				})
		})
}
