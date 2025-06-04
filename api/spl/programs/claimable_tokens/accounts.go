package claimable_tokens

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

func deriveNonce(ethAddress common.Address, authority solana.PublicKey) (solana.PublicKey, uint8, error) {
	nonceSeedPrefix := []byte(NonceSeedPrefix)
	decodedEthAddress := ethAddress.Bytes()
	seed := make([]byte, len(nonceSeedPrefix)+EthAddressByteLength)
	copy(seed, nonceSeedPrefix)
	copy(seed[len(seed)-len(decodedEthAddress):], decodedEthAddress)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[:32], seed}, ProgramID)
}

func deriveAuthority(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress([][]byte{mint.Bytes()[:32]}, ProgramID)
}

func deriveUserBankAccount(mint solana.PublicKey, ethAddress common.Address) (solana.PublicKey, error) {
	ethAddressBytes := ethAddress.Bytes()
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
