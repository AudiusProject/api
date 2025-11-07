package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go"
	"golang.org/x/crypto/sha3"
)

// DeriveEthAddressForMint derives a stable secp256k1 private key from (domain, secret, mint)
// and returns both the ETH address (EIP-55) and private key (hex).
// Uses keccak256(domain || secret || mint || ctr) and retries with a tiny counter until valid.
func DeriveEthAddressForMint(domain []byte, secretHex string, mint solana.PublicKey) (address string, privateKey string, err error) {
	// Remove 0x prefix if present
	h := strings.TrimPrefix(secretHex, "0x")
	if len(h) != 64 {
		return "", "", errors.New("secret must be 32-byte hex")
	}

	secret, err := hex.DecodeString(h)
	if err != nil {
		return "", "", fmt.Errorf("invalid secret hex: %w", err)
	}

	mintBytes := mint.Bytes()

	for ctr := 0; ctr < 16; ctr++ {
		// Concatenate domain + secret + mintBytes + counter
		data := make([]byte, 0, len(domain)+len(secret)+len(mintBytes)+1)
		data = append(data, domain...)
		data = append(data, secret...)
		data = append(data, mintBytes...)
		data = append(data, byte(ctr))

		// Hash with keccak256
		hash := sha3.NewLegacyKeccak256()
		hash.Write(data)
		priv := hash.Sum(nil)

		// Verify it's a valid secp256k1 private key
		privKey, err := crypto.ToECDSA(priv)
		if err != nil {
			continue // Invalid key, try next counter
		}

		// Get uncompressed public key (65 bytes: 0x04 + X(32) + Y(32))
		pubKey := crypto.FromECDSAPub(&privKey.PublicKey)

		// Hash the public key (without the 0x04 prefix) with keccak256
		pubKeyHash := sha3.NewLegacyKeccak256()
		pubKeyHash.Write(pubKey[1:]) // Drop the 0x04 prefix
		pubKeyHashBytes := pubKeyHash.Sum(nil)

		// Take the last 20 bytes as the Ethereum address
		addrBytes := pubKeyHashBytes[len(pubKeyHashBytes)-20:]
		addr := common.BytesToAddress(addrBytes)

		// Convert to EIP-55 checksum address
		address = addr.Hex() // This already returns checksummed address
		privateKey = hex.EncodeToString(priv)

		return address, privateKey, nil
	}

	return "", "", errors.New("could not derive a valid secp256k1 key")
}
