package payment_router

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/gagliardetto/solana-go/text/format"
	"github.com/gagliardetto/treeout"
)

type Route struct {
	PaymentRouterPdaBump uint8
	Amounts              []uint64
	TotalAmount          uint64

	Accounts solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*Route)(nil)
	_ solana.AccountsSettable = (*Route)(nil)
	_ text.EncodableToTree    = (*Route)(nil)
)

func NewRouteInstructionBuilder() *Route {
	inst := &Route{
		Accounts: make(solana.AccountMetaSlice, 3),
	}
	inst.Accounts[2] = solana.Meta(solana.TokenProgramID)
	return inst
}

func (inst *Route) SetSender(sender solana.PublicKey) *Route {
	inst.Accounts[0] = solana.Meta(sender).WRITE()
	return inst
}

func (inst *Route) GetSender() *solana.AccountMeta {
	return inst.Accounts.Get(0)
}

func (inst *Route) SetSenderOwner(senderOwner solana.PublicKey) *Route {
	inst.Accounts[1] = solana.Meta(senderOwner)
	return inst
}

func (inst *Route) GetSenderOwner() *solana.AccountMeta {
	return inst.Accounts.Get(1)
}

func (inst *Route) GetDestinations() solana.AccountMetaSlice {
	return inst.Accounts[3:]
}

func (inst *Route) SetPaymentRouterPdaBump(paymentRouterPdaBump uint8) *Route {
	inst.PaymentRouterPdaBump = paymentRouterPdaBump
	return inst
}

func (inst *Route) AddRoute(destination solana.PublicKey, amount uint64) *Route {
	inst.Accounts.Append(solana.Meta(destination).WRITE())
	inst.Amounts = append(inst.Amounts, amount)
	inst.TotalAmount += amount
	return inst
}

func (inst *Route) SetRouteMap(routeMap map[solana.PublicKey]uint64) *Route {
	inst.Accounts = inst.Accounts[:3]
	inst.Amounts = make([]uint64, 0)
	inst.TotalAmount = 0
	for key, val := range routeMap {
		inst.AddRoute(key, val)
	}
	return inst
}

func (inst *Route) GetRouteMap() map[solana.PublicKey]uint64 {
	res := make(map[solana.PublicKey]uint64)
	for i, acc := range inst.GetDestinations() {
		res[acc.PublicKey] = inst.Amounts[i]
	}
	return res
}

func (inst *Route) Build() *Instruction {
	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromSighash(bin.SighashInstruction(Instruction_Route)),
	}}
}

// ----- solana.AccountsSettable Implementation -----

func (inst *Route) SetAccounts(accounts []*solana.AccountMeta) error {
	return inst.Accounts.SetAccounts(accounts)
}

// ----- solana.AccountsGettable Implementation -----

func (inst Route) GetAccounts() []*solana.AccountMeta {
	return inst.Accounts
}

// ----- text.EncodableToTree Implementation -----

func (inst *Route) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program("PaymentRouter", ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction(Instruction_Route)).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch treeout.Branches) {
						paramsBranch.Child(format.Param("PdaBump", inst.PaymentRouterPdaBump))
						paramsBranch.Child(format.Param("Amounts", inst.Amounts))
						paramsBranch.Child(format.Param("TotalAmount", inst.TotalAmount))
					})
					instructionBranch.Child("Accounts").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Account("sender", inst.GetSender().PublicKey))
						accountsBranch.Child(format.Account("senderOwner", inst.GetSenderOwner().PublicKey))
						for i, dest := range inst.GetDestinations() {
							accountsBranch.Child(format.Account(fmt.Sprintf("destination %d", i), dest.PublicKey))
						}
					})
					instructionBranch.Child("Routes").ParentFunc(func(routesBranch treeout.Branches) {
						for pubkey, amount := range inst.GetRouteMap() {
							routesBranch.Child(format.Account("Account", pubkey))
							routesBranch.Child(format.Param("Amount", amount))
						}
					})
				})
		})
}

func NewRouteInstruction(
	sender solana.PublicKey,
	senderOwner solana.PublicKey,
	paymentRouterPdaBump uint8,
	routeMap map[solana.PublicKey]uint64,
) *Route {
	return NewRouteInstructionBuilder().
		SetSender(sender).
		SetSenderOwner(senderOwner).
		SetPaymentRouterPdaBump(paymentRouterPdaBump).
		SetRouteMap(routeMap)
}
