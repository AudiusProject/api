package address_lookup_table

import (
	"errors"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

// CreateLookupTable creates a new address lookup table
type CreateLookupTable struct {
	RecentSlot uint64

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*CreateLookupTable)(nil)
	_ solana.AccountsSettable = (*CreateLookupTable)(nil)
	_ text.EncodableToTree    = (*CreateLookupTable)(nil)
)

func NewCreateLookupTableInstructionBuilder() *CreateLookupTable {
	return &CreateLookupTable{
		Accounts: make(solana.AccountMetaSlice, 4),
	}
}

func (inst *CreateLookupTable) SetRecentSlot(recentSlot uint64) *CreateLookupTable {
	inst.RecentSlot = recentSlot
	return inst
}

func (inst *CreateLookupTable) SetLookupTableAccount(lookupTable solana.PublicKey) *CreateLookupTable {
	inst.Accounts[0] = solana.Meta(lookupTable).WRITE()
	return inst
}

func (inst *CreateLookupTable) LookupTableAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *CreateLookupTable) SetAuthorityAccount(authority solana.PublicKey) *CreateLookupTable {
	inst.Accounts[1] = solana.Meta(authority).SIGNER()
	return inst
}

func (inst *CreateLookupTable) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *CreateLookupTable) SetPayerAccount(payer solana.PublicKey) *CreateLookupTable {
	inst.Accounts[2] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *CreateLookupTable) PayerAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *CreateLookupTable) SetSystemProgramAccount(systemProgram solana.PublicKey) *CreateLookupTable {
	inst.Accounts[3] = solana.Meta(systemProgram)
	return inst
}

func (inst *CreateLookupTable) SystemProgramAccount() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *CreateLookupTable) Validate() error {
	if inst.RecentSlot == 0 {
		return errors.New("recentSlot not set")
	}
	if inst.LookupTableAccount() == nil {
		return errors.New("lookupTable account not set")
	}
	if inst.AuthorityAccount() == nil {
		return errors.New("authority account not set")
	}
	if inst.PayerAccount() == nil {
		return errors.New("payer account not set")
	}
	if inst.SystemProgramAccount() == nil {
		return errors.New("systemProgram account not set")
	}
	return nil
}

func (inst *CreateLookupTable) Build() *solana.GenericInstruction {
	return &solana.GenericInstruction{
		ProgID:        ProgramID,
		AccountValues: inst.Accounts,
		DataBytes:     inst.data(),
	}
}

func (inst *CreateLookupTable) data() []byte {
	data := make([]byte, 12)
	bin.LE.PutUint32(data[0:4], Instruction_CreateLookupTable)
	bin.LE.PutUint64(data[4:12], inst.RecentSlot)
	return data
}

func (inst *CreateLookupTable) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

func (inst CreateLookupTable) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

func (inst *CreateLookupTable) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("CreateLookupTable", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Param("RecentSlot", inst.RecentSlot))
			programBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
				if inst.LookupTableAccount() != nil {
					accountsBranch.Child(format.Meta("lookupTable", inst.LookupTableAccount()))
				}
				if inst.AuthorityAccount() != nil {
					accountsBranch.Child(format.Meta("authority", inst.AuthorityAccount()))
				}
				if inst.PayerAccount() != nil {
					accountsBranch.Child(format.Meta("payer", inst.PayerAccount()))
				}
				if inst.SystemProgramAccount() != nil {
					accountsBranch.Child(format.Meta("systemProgram", inst.SystemProgramAccount()))
				}
			})
		})
}

// NewCreateLookupTableInstruction creates a new CreateLookupTable instruction
func NewCreateLookupTableInstruction(
	recentSlot uint64,
	lookupTable solana.PublicKey,
	authority solana.PublicKey,
	payer solana.PublicKey,
) *CreateLookupTable {
	return NewCreateLookupTableInstructionBuilder().
		SetRecentSlot(recentSlot).
		SetLookupTableAccount(lookupTable).
		SetAuthorityAccount(authority).
		SetPayerAccount(payer).
		SetSystemProgramAccount(solana.SystemProgramID)
}
