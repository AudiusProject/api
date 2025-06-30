package payment_router

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type Route struct {
	Sender      solana.PublicKey
	SenderOwner solana.PublicKey

	Destinations solana.PublicKeySlice

	Data RouteData

	solana.AccountMetaSlice
}

type RouteData struct {
	PaymentRouterPdaBump uint8
	Amounts              []uint64
	TotalAmount          uint64
}

func (inst *Route) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	sender := inst.AccountMetaSlice.Get(0)
	if sender != nil {
		inst.Sender = sender.PublicKey
	}
	senderOwner := inst.AccountMetaSlice.Get(1)
	if senderOwner != nil {
		inst.SenderOwner = senderOwner.PublicKey
	}
	if len(accounts) > 3 {
		inst.Destinations = inst.GetKeys()[3:]
	}
	return nil
}

func (inst Route) MarshalWithEncoder(encoder *bin.Encoder) error {
	return encoder.Encode(inst.Data)
}

func (inst *Route) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return decoder.Decode(&inst.Data)
}
