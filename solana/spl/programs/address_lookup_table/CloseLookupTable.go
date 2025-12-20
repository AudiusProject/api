package address_lookup_table

import (
	"errors"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

// CloseLookupTable closes a deactivated lookup table and reclaims rent
type CloseLookupTable struct {
	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*CloseLookupTable)(nil)
	_ solana.AccountsSettable = (*CloseLookupTable)(nil)
	_ text.EncodableToTree    = (*CloseLookupTable)(nil)
)

func NewCloseLookupTableInstructionBuilder() *CloseLookupTable {
	return &CloseLookupTable{
		Accounts: make(solana.AccountMetaSlice, 3),
	}
}

func (inst *CloseLookupTable) SetLookupTableAccount(lookupTable solana.PublicKey) *CloseLookupTable {
	inst.Accounts[0] = solana.Meta(lookupTable).WRITE()
	return inst
}

func (inst *CloseLookupTable) LookupTableAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *CloseLookupTable) SetAuthorityAccount(authority solana.PublicKey) *CloseLookupTable {
	inst.Accounts[1] = solana.Meta(authority).SIGNER()
	return inst
}

func (inst *CloseLookupTable) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *CloseLookupTable) SetRecipientAccount(recipient solana.PublicKey) *CloseLookupTable {
	inst.Accounts[2] = solana.Meta(recipient).WRITE()
	return inst
}

func (inst *CloseLookupTable) RecipientAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *CloseLookupTable) Validate() error {
	if inst.LookupTableAccount() == nil {
		return errors.New("lookupTable account not set")
	}
	if inst.AuthorityAccount() == nil {
		return errors.New("authority account not set")
	}
	if inst.RecipientAccount() == nil {
		return errors.New("recipient account not set")
	}
	return nil
}

func (inst *CloseLookupTable) Build() *solana.GenericInstruction {
	return &solana.GenericInstruction{
		ProgID:        ProgramID,
		AccountValues: inst.Accounts,
		DataBytes:     inst.data(),
	}
}

func (inst *CloseLookupTable) data() []byte {
	data := make([]byte, 4)
	bin.LE.PutUint32(data[0:4], Instruction_CloseLookupTable)
	return data
}

func (inst *CloseLookupTable) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

func (inst CloseLookupTable) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

func (inst *CloseLookupTable) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("CloseLookupTable", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
				if inst.LookupTableAccount() != nil {
					accountsBranch.Child(format.Meta("lookupTable", inst.LookupTableAccount()))
				}
				if inst.AuthorityAccount() != nil {
					accountsBranch.Child(format.Meta("authority", inst.AuthorityAccount()))
				}
				if inst.RecipientAccount() != nil {
					accountsBranch.Child(format.Meta("recipient", inst.RecipientAccount()))
				}
			})
		})
}

// NewCloseLookupTableInstruction creates a new CloseLookupTable instruction
func NewCloseLookupTableInstruction(
	lookupTable solana.PublicKey,
	authority solana.PublicKey,
	recipient solana.PublicKey,
) *CloseLookupTable {
	return NewCloseLookupTableInstructionBuilder().
		SetLookupTableAccount(lookupTable).
		SetAuthorityAccount(authority).
		SetRecipientAccount(recipient)
}
