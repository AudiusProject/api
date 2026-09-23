package claimable_tokens

import "github.com/gagliardetto/solana-go"

// SetAuthority has no instruction data beyond its variant discriminator.
// The signed SPL Token set-authority payload is carried by the preceding
// secp256k1 instruction.
type SetAuthority struct {
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

var (
	_ solana.AccountsGettable = (*SetAuthority)(nil)
	_ solana.AccountsSettable = (*SetAuthority)(nil)
)
