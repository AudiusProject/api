package claimable_tokens

import (
	"encoding/hex"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

func deriveAuthority(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress([][]byte{mint.Bytes()[:32]}, ProgramID)
}

func DeriveUserBankAccount(mint solana.PublicKey, ethAddress string) (solana.PublicKey, error) {
	ethAddressBytes, err := hex.DecodeString(strings.TrimPrefix(ethAddress, "0x"))
	if err != nil {
		return solana.PublicKey{}, err
	}

	seed := base58.Encode(ethAddressBytes)
	authority, _, err := deriveAuthority(mint)
	if err != nil {
		return solana.PublicKey{}, err
	}

	pubkey, err := solana.CreateWithSeed(authority, seed, solana.TokenProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}

	return pubkey, nil
}
