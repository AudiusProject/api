package meteora_damm_v2

import "github.com/gagliardetto/solana-go"

// Derives the position PDA from a position NFT mint
func DerivePositionPDA(positionNft solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("position"), positionNft.Bytes()}
	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return address, nil
}
