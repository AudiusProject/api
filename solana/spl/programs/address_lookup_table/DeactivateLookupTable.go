package address_lookup_table

import (
	"errors"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

// DeactivateLookupTable deactivates a lookup table so it can be closed
type DeactivateLookupTable struct {
	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*DeactivateLookupTable)(nil)
	_ solana.AccountsSettable = (*DeactivateLookupTable)(nil)
	_ text.EncodableToTree    = (*DeactivateLookupTable)(nil)
)

func NewDeactivateLookupTableInstructionBuilder() *DeactivateLookupTable {
	return &DeactivateLookupTable{
		Accounts: make(solana.AccountMetaSlice, 2),
	}
}

func (inst *DeactivateLookupTable) SetLookupTableAccount(lookupTable solana.PublicKey) *DeactivateLookupTable {
	inst.Accounts[0] = solana.Meta(lookupTable).WRITE()
	return inst
}

func (inst *DeactivateLookupTable) LookupTableAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *DeactivateLookupTable) SetAuthorityAccount(authority solana.PublicKey) *DeactivateLookupTable {
	inst.Accounts[1] = solana.Meta(authority).SIGNER()
	return inst
}

func (inst *DeactivateLookupTable) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *DeactivateLookupTable) Validate() error {
	if inst.LookupTableAccount() == nil {
		return errors.New("lookupTable account not set")
	}
	if inst.AuthorityAccount() == nil {
		return errors.New("authority account not set")
	}
	return nil
}

func (inst *DeactivateLookupTable) Build() *solana.GenericInstruction {
	return &solana.GenericInstruction{
		ProgID:        ProgramID,
		AccountValues: inst.Accounts,
		DataBytes:     inst.data(),
	}
}

func (inst *DeactivateLookupTable) data() []byte {
	data := make([]byte, 4)
	bin.LE.PutUint32(data[0:4], Instruction_DeactivateLookupTable)
	return data
}

func (inst *DeactivateLookupTable) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

func (inst DeactivateLookupTable) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

func (inst *DeactivateLookupTable) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("DeactivateLookupTable", ProgramID)).
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

// NewDeactivateLookupTableInstruction creates a new DeactivateLookupTable instruction
func NewDeactivateLookupTableInstruction(
	lookupTable solana.PublicKey,
	authority solana.PublicKey,
) *DeactivateLookupTable {
	return NewDeactivateLookupTableInstructionBuilder().
		SetLookupTableAccount(lookupTable).
		SetAuthorityAccount(authority)
}
