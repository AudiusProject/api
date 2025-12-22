package address_lookup_table

import (
	"errors"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

// ExtendLookupTable extends an existing lookup table with new addresses
type ExtendLookupTable struct {
	NewAddresses []solana.PublicKey

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*ExtendLookupTable)(nil)
	_ solana.AccountsSettable = (*ExtendLookupTable)(nil)
	_ text.EncodableToTree    = (*ExtendLookupTable)(nil)
)

func NewExtendLookupTableInstructionBuilder() *ExtendLookupTable {
	return &ExtendLookupTable{
		Accounts: make(solana.AccountMetaSlice, 4),
	}
}

func (inst *ExtendLookupTable) SetNewAddresses(addresses []solana.PublicKey) *ExtendLookupTable {
	inst.NewAddresses = addresses
	return inst
}

func (inst *ExtendLookupTable) SetLookupTableAccount(lookupTable solana.PublicKey) *ExtendLookupTable {
	inst.Accounts[0] = solana.Meta(lookupTable).WRITE()
	return inst
}

func (inst *ExtendLookupTable) LookupTableAccount() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *ExtendLookupTable) SetAuthorityAccount(authority solana.PublicKey) *ExtendLookupTable {
	inst.Accounts[1] = solana.Meta(authority).SIGNER()
	return inst
}

func (inst *ExtendLookupTable) AuthorityAccount() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *ExtendLookupTable) SetPayerAccount(payer solana.PublicKey) *ExtendLookupTable {
	inst.Accounts[2] = solana.Meta(payer).SIGNER().WRITE()
	return inst
}

func (inst *ExtendLookupTable) PayerAccount() *solana.AccountMeta {
	return inst.Accounts.Get(2)
}

func (inst *ExtendLookupTable) SetSystemProgramAccount(systemProgram solana.PublicKey) *ExtendLookupTable {
	inst.Accounts[3] = solana.Meta(systemProgram)
	return inst
}

func (inst *ExtendLookupTable) SystemProgramAccount() *solana.AccountMeta {
	return inst.Accounts.Get(3)
}

func (inst *ExtendLookupTable) Validate() error {
	if len(inst.NewAddresses) == 0 {
		return errors.New("newAddresses not set")
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

func (inst *ExtendLookupTable) Build() *solana.GenericInstruction {
	return &solana.GenericInstruction{
		ProgID:        ProgramID,
		AccountValues: inst.Accounts,
		DataBytes:     inst.data(),
	}
}

func (inst *ExtendLookupTable) data() []byte {
	// Instruction discriminator (4 bytes) + num addresses (8 bytes) + addresses
	numAddresses := uint64(len(inst.NewAddresses))
	data := make([]byte, 4+8+(numAddresses*32))

	bin.LE.PutUint32(data[0:4], Instruction_ExtendLookupTable)
	bin.LE.PutUint64(data[4:12], numAddresses)

	offset := 12
	for _, addr := range inst.NewAddresses {
		copy(data[offset:offset+32], addr[:])
		offset += 32
	}

	return data
}

func (inst *ExtendLookupTable) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

func (inst ExtendLookupTable) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

func (inst *ExtendLookupTable) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("ExtendLookupTable", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Param("NumAddresses", len(inst.NewAddresses)))
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

// NewExtendLookupTableInstruction creates a new ExtendLookupTable instruction
func NewExtendLookupTableInstruction(
	authority solana.PublicKey,
	lookupTable solana.PublicKey,
	payer solana.PublicKey,
	newAddresses []solana.PublicKey,
) *ExtendLookupTable {
	return NewExtendLookupTableInstructionBuilder().
		SetNewAddresses(newAddresses).
		SetLookupTableAccount(lookupTable).
		SetAuthorityAccount(authority).
		SetPayerAccount(payer).
		SetSystemProgramAccount(solana.SystemProgramID)
}
