package main

import (
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// Check if private key was provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run verify_key.go <private_key>")
		return
	}

	// Get private key from command line argument
	privateKeyHex := os.Args[1]

	// Try to load the private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		fmt.Printf("Error loading private key: %v\n", err)
		return
	}

	// Get the public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Println("Error casting public key to ECDSA")
		return
	}

	// Get the Ethereum address
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Printf("Private key corresponds to address: %s\n", address)
}
