package spl

import (
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type AddressLookupTableMeta struct {
	DeactivationSlot           uint64
	LastExtendedSlot           uint64
	LastExtendedSlotStartIndex uint8
	Authority                  []solana.PublicKey
}

type AddressLookupTable struct {
	State     uint32
	Meta      AddressLookupTableMeta
	Addresses []solana.PublicKey
}

func (inst *AddressLookupTable) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	err := decoder.Decode(&inst.State)
	if err != nil {
		return err
	}
	err = decoder.Decode(&inst.Meta)
	if err != nil {
		return err
	}
	err = decoder.SetPosition(56)
	if err != nil {
		return err
	}
	for decoder.HasRemaining() {
		pub := &solana.PublicKey{}
		err = decoder.Decode(pub)
		if err != nil {
			return err
		}
		inst.Addresses = append(inst.Addresses, *pub)
	}
	return err
}
