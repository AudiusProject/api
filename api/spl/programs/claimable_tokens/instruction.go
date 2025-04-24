package claimable_tokens

import "github.com/gagliardetto/solana-go"

var ProgramID = solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ")

func SetProgramID(pubkey solana.PublicKey) {
	ProgramID = pubkey
}
