package utils

import (
	"crypto/ed25519"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestDeriveRewardManagerKeypair(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R")
	secret := "0000000000000000000000000000000000000000000000000000000000000001"

	priv := DeriveRewardManagerKeypair(secret, mint)

	t.Run("returns a well-formed ed25519 private key", func(t *testing.T) {
		if len(priv) != ed25519.PrivateKeySize {
			t.Fatalf("private key length = %d, want %d", len(priv), ed25519.PrivateKeySize)
		}
		pub, ok := priv.Public().(ed25519.PublicKey)
		if !ok {
			t.Fatalf("priv.Public() did not return ed25519.PublicKey")
		}
		if len(pub) != ed25519.PublicKeySize {
			t.Fatalf("public key length = %d, want %d", len(pub), ed25519.PublicKeySize)
		}
	})

	t.Run("derivation is deterministic", func(t *testing.T) {
		again := DeriveRewardManagerKeypair(secret, mint)
		if string(again) != string(priv) {
			t.Fatalf("re-derivation produced a different private key")
		}
	})

	t.Run("signs and verifies", func(t *testing.T) {
		msg := []byte("smoke test")
		sig := ed25519.Sign(priv, msg)
		if !ed25519.Verify(priv.Public().(ed25519.PublicKey), msg, sig) {
			t.Fatalf("signature did not verify against the derived public key")
		}
	})

	t.Run("different mints produce different keypairs", func(t *testing.T) {
		otherMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
		other := DeriveRewardManagerKeypair(secret, otherMint)
		if string(other) == string(priv) {
			t.Fatalf("different mints should yield different RM keypairs")
		}
	})

	t.Run("different secrets produce different keypairs", func(t *testing.T) {
		otherSecret := "0000000000000000000000000000000000000000000000000000000000000002"
		other := DeriveRewardManagerKeypair(otherSecret, mint)
		if string(other) == string(priv) {
			t.Fatalf("different secrets should yield different RM keypairs")
		}
	})
}
