package utils

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveEthAddressForMint(t *testing.T) {
	t.Run("Successfully derives address and private key", func(t *testing.T) {
		// Test with a valid domain, secret, and mint
		domain := []byte("test-domain")
		secretHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		address, privateKey, err := DeriveEthAddressForMint(domain, secretHex, mint)
		require.NoError(t, err)

		// Verify address format (0x + 40 hex chars)
		assert.True(t, strings.HasPrefix(address, "0x"))
		assert.Len(t, address, 42)

		// Verify private key format (64 hex chars)
		assert.Len(t, privateKey, 64)

		// Verify address is valid hex
		_, err = hex.DecodeString(strings.TrimPrefix(address, "0x"))
		assert.NoError(t, err)

		// Verify private key is valid hex
		_, err = hex.DecodeString(strings.TrimPrefix(privateKey, "0x"))
		assert.NoError(t, err)

		// Verify address has proper checksum (uppercase/lowercase)
		// EIP-55 addresses have mixed case
		hasUppercase := strings.ToUpper(address) != address
		hasLowercase := strings.ToLower(address) != address
		assert.True(t, hasUppercase && hasLowercase, "Address should be checksummed")
	})

	t.Run("Works with secret without 0x prefix", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		address, privateKey, err := DeriveEthAddressForMint(domain, secretHex, mint)
		require.NoError(t, err)
		assert.NotEmpty(t, address)
		assert.NotEmpty(t, privateKey)
	})

	t.Run("Works with secret with 0x prefix", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		address, privateKey, err := DeriveEthAddressForMint(domain, secretHex, mint)
		require.NoError(t, err)
		assert.NotEmpty(t, address)
		assert.NotEmpty(t, privateKey)
	})

	t.Run("Deterministic output for same inputs", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		address1, privateKey1, err := DeriveEthAddressForMint(domain, secretHex, mint)
		require.NoError(t, err)

		address2, privateKey2, err := DeriveEthAddressForMint(domain, secretHex, mint)
		require.NoError(t, err)

		assert.Equal(t, address1, address2, "Should generate same address")
		assert.Equal(t, privateKey1, privateKey2, "Should generate same private key")
	})

	t.Run("Different outputs for different mints", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint1 := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
		mint2 := solana.MustPublicKeyFromBase58("DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq")

		address1, privateKey1, err := DeriveEthAddressForMint(domain, secretHex, mint1)
		require.NoError(t, err)

		address2, privateKey2, err := DeriveEthAddressForMint(domain, secretHex, mint2)
		require.NoError(t, err)

		assert.NotEqual(t, address1, address2, "Different mints should generate different addresses")
		assert.NotEqual(t, privateKey1, privateKey2, "Different mints should generate different private keys")
	})

	t.Run("Returns error for invalid secret length", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "0123456789abcdef" // Too short
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		_, _, err := DeriveEthAddressForMint(domain, secretHex, mint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "secret must be 32-byte hex")
	})

	t.Run("Returns error for invalid secret hex", func(t *testing.T) {
		domain := []byte("test-domain")
		secretHex := "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz" // Invalid hex
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		_, _, err := DeriveEthAddressForMint(domain, secretHex, mint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid secret hex")
	})

	t.Run("Different domains produce different results", func(t *testing.T) {
		domain1 := []byte("domain1")
		domain2 := []byte("domain2")
		secretHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")

		address1, privateKey1, err := DeriveEthAddressForMint(domain1, secretHex, mint)
		require.NoError(t, err)

		address2, privateKey2, err := DeriveEthAddressForMint(domain2, secretHex, mint)
		require.NoError(t, err)

		assert.NotEqual(t, address1, address2, "Different domains should generate different addresses")
		assert.NotEqual(t, privateKey1, privateKey2, "Different domains should generate different private keys")
	})
}
