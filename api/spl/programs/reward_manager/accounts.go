package reward_manager

import (
	"encoding/hex"
	"strings"

	"github.com/gagliardetto/solana-go"
)

type RewardManagerState struct {
	Version      uint8
	TokenAccount solana.PublicKey
	Manager      solana.PublicKey
	MinVotes     uint8
}

func deriveAuthority(programId solana.PublicKey, state solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := make([][]byte, 1)
	seeds[0] = state.Bytes()[0:32]
	return solana.FindProgramAddress(seeds, programId)
}

func deriveSender(programId solana.PublicKey, authority solana.PublicKey, ethAddress string) (solana.PublicKey, uint8, error) {
	senderSeedPrefix := []byte(SenderSeedPrefix)
	// Remove 0x and decode hex
	decodedEthAddress, err := hex.DecodeString(strings.TrimPrefix(ethAddress, "0x"))
	if err != nil {
		return solana.PublicKey{}, 0, err
	}
	// Pad the eth address if necessary w/ leading 0
	senderSeed := make([]byte, len(senderSeedPrefix)+EthAddressByteLength)
	copy(senderSeed, senderSeedPrefix)
	copy(senderSeed[len(senderSeed)-len(decodedEthAddress):], decodedEthAddress)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], senderSeed}, programId)
}

func deriveAttestations(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	attestationsSeed := make([]byte, len(AttestationsSeedPrefix)+len(disbursementId))
	copy(attestationsSeed, []byte(AttestationsSeedPrefix))
	copy(attestationsSeed[len([]byte(AttestationsSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], attestationsSeed}, programId)
}

func deriveDisbursement(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	disbursementSeed := make([]byte, len(DisbursementSeedPrefix)+len(disbursementId))
	copy(disbursementSeed, []byte(DisbursementSeedPrefix))
	copy(disbursementSeed[len([]byte(DisbursementSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], disbursementSeed}, programId)
}
