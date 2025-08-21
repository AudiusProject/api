package testdata

import (
	"crypto/ecdsa"
	"encoding/base64"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestWallet represents a test wallet with private key and address
type TestWallet struct {
	PrivateKey *ecdsa.PrivateKey
	Address    string
}

// CreateTestWallet creates a new test wallet from a private key hex string
func CreateTestWallet(privateKeyHex string) (*TestWallet, error) {
	// Remove "0x" prefix if present
	if len(privateKeyHex) > 2 && privateKeyHex[:2] == "0x" {
		privateKeyHex = privateKeyHex[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, err
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &TestWallet{
		PrivateKey: privateKey,
		Address:    address.Hex(),
	}, nil
}

// SignData signs the given data with the test wallet's private key
// Returns the base64-encoded signature that can be used with ReadSignedPost
func (w *TestWallet) SignData(data []byte) (string, error) {
	// Hash the data using the same method as recoverSigningWallet
	hash := crypto.Keccak256Hash(data)

	// Sign the hash
	signature, err := crypto.Sign(hash[:], w.PrivateKey)
	if err != nil {
		return "", err
	}

	// Base64 encode the signature (same as expected by recoverSigningWallet)
	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	return signatureBase64, nil
}
