package meteora_damm_v2

import "github.com/gagliardetto/solana-go"

var ProgramID = solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")

func SetProgramID(pubkey solana.PublicKey) {
	ProgramID = pubkey
}
