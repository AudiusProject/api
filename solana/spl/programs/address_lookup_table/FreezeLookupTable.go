package address_lookup_table

import (
	"errors"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

// FreezeLookupTable freezes a lookup table making it immutable
type FreezeLookupTable struct {
	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*FreezeLookupTable)(nil)
	_ solana.AccountsSettable = (*FreezeLookupTable)(nil)
	_ text.EncodableToTree    = (*FreezeLookupTable)(nil)
)

func NewFreezeLookupTableInstructionBuilder() *FreezeLookupTable {
	return &FreezeLookupTable{
		Accounts: make(solana.AccountMetaSlice, 2),
	}
}

func (inst *FreezeLookupTable) SetLookupTableAccount(lookupTable solana.PublicKey) *FreezeLookupTable {
	inst.Accounts[0] = solana.Meta(lookupTable).WRITE()
	return inst
}

func (inst *FreezeLookupTable) LookupTableAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *FreezeLookupTable) SetAuthorityAccount(authority solana.PublicKey) *FreezeLookupTable {
	inst.Accounts[1] = solana.Meta(authority).SIGNER()
	return inst
}

func (inst *FreezeLookupTable) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *FreezeLookupTable) Validate() error {
	if inst.LookupTableAccount() == nil {
		return errors.New("lookupTable account not set")
	}
	if inst.AuthorityAccount() == nil {
		return errors.New("authority account not set")
	}
	return nil
}

func (inst *FreezeLookupTable) Build() *solana.GenericInstruction {
	return &solana.GenericInstruction{
		ProgID:        ProgramID,
		AccountValues: inst.Accounts,
		DataBytes:     inst.data(),
	}
}

func (inst *FreezeLookupTable) data() []byte {
	data := make([]byte, 4)
	bin.LE.PutUint32(data[0:4], Instruction_FreezeLookupTable)
	return data
}

func (inst *FreezeLookupTable) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

func (inst FreezeLookupTable) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

func (inst *FreezeLookupTable) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("FreezeLookupTable", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
				if inst.LookupTableAccount() != nil {
					accountsBranch.Child(format.Meta("lookupTable", inst.LookupTableAccount()))
				}
				if inst.AuthorityAccount() != nil {
					accountsBranch.Child(format.Meta("authority", inst.AuthorityAccount()))
				}
			})
		})
}

// NewFreezeLookupTableInstruction creates a new FreezeLookupTable instruction
func NewFreezeLookupTableInstruction(
	lookupTable solana.PublicKey,
	authority solana.PublicKey,
) *FreezeLookupTable {
	return NewFreezeLookupTableInstructionBuilder().
		SetLookupTableAccount(lookupTable).
		SetAuthorityAccount(authority)
}
