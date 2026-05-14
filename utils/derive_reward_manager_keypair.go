package utils

import (
	"crypto/ed25519"
	"crypto/sha256"

	"github.com/gagliardetto/solana-go"
)

// DeriveRewardManagerKeypair deterministically derives the ed25519 keypair
// for the Solana reward manager state account associated with a launchpad
// mint. The result matches the keypair produced by the solana-relay's
// `deriveKeypair('reward-manager', mint)` helper (see
// apps/packages/discovery-provider/plugins/pedalboard/apps/solana-relay/
// src/routes/launchpad/launch_coin.ts), so the public key equals the
// rewards_manager_pubkey that cometbft carries for that mint's pool.
//
// Seed material:
//
//	sha256(secret_utf8 || "audius-launchpad" || "reward-manager" || mint_bytes)
//
// where secret_utf8 is the UTF-8 bytes of the launchpad's hex-encoded
// secret STRING (NOT the decoded hex bytes — matches the TS
// `Buffer.from(secret, 'utf8')`).
//
// The returned private key is what callers feed to
// `oap.Rewards.CreateRewardPool` as the `rmKey` argument: the cometbft
// validator verifies the envelope's rm_owner_signature against the
// matching public key, which prevents an observer of Solana RM init
// events from frontrunning pool creation with attacker-chosen
// authorities.
func DeriveRewardManagerKeypair(secretHex string, mint solana.PublicKey) ed25519.PrivateKey {
	var buf []byte
	buf = append(buf, []byte(secretHex)...)
	buf = append(buf, []byte("audius-launchpad")...)
	buf = append(buf, []byte("reward-manager")...)
	buf = append(buf, mint.Bytes()...)
	seed := sha256.Sum256(buf)
	return ed25519.NewKeyFromSeed(seed[:])
}
