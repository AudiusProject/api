package meteora_dbc

import (
	"api.audius.co/solana/spl/programs/meteora_locker"
	"github.com/gagliardetto/solana-go"
)

func DeriveEscrow(base solana.PublicKey) solana.PublicKey {
	escrowSeed := []byte("escrow")
	escrowPubkey, _, err := solana.FindProgramAddress(
		[][]byte{
			escrowSeed,
			base.Bytes(),
		},
		meteora_locker.ProgramID,
	)
	if err != nil {
		panic(err)
	}
	return escrowPubkey
}

func DeriveBaseKeyForEscrow(pool solana.PublicKey) solana.PublicKey {
	baseLockerSeed := []byte("base_locker")
	baseKeyPubkey, _, err := solana.FindProgramAddress(
		[][]byte{
			baseLockerSeed,
			pool.Bytes(),
		},
		ProgramID,
	)
	if err != nil {
		panic(err)
	}
	return baseKeyPubkey
}
