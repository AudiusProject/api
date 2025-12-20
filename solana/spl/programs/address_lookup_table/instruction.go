package address_lookup_table

import (
"github.com/gagliardetto/solana-go"
)

// ProgramID is the address of the Address Lookup Table program
var ProgramID = solana.MustPublicKeyFromBase58("AddressLookupTab1e1111111111111111111111111")

const (
Instruction_CreateLookupTable uint32 = iota
Instruction_FreezeLookupTable
Instruction_ExtendLookupTable
Instruction_DeactivateLookupTable
Instruction_CloseLookupTable
)
