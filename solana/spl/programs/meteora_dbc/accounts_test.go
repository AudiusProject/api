package meteora_dbc

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
)

func TestDeriveEscrow(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("5AB7G5jwzfLCE5CNimiPUBsWARxa5PyhBncGnp9HnVN9")
	expectedEscrowPubkey := solana.MustPublicKeyFromBase58("FN4jywYdBTks9iMk9dozp5WzY5dHxQ6pmF2H6DH5Gp7V")

	baseKey := DeriveBaseKeyForEscrow(pool)
	derivedEscrowPubkey := DeriveEscrow(baseKey)
	assert.Equal(t, expectedEscrowPubkey, derivedEscrowPubkey)
}
