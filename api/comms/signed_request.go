package comms

import (
	"crypto/ecdsa"
	"encoding/base64"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

func RecoverSigningWallet(signatureHex string, signedData []byte) (string, *ecdsa.PublicKey, error) {
	sig, err := base64.StdEncoding.DecodeString(signatureHex)
	if err != nil {
		err = errors.New("bad sig header: " + err.Error())
		return "", nil, err
	}

	// recover
	hash := crypto.Keccak256Hash(signedData)
	pubkey, err := crypto.SigToPub(hash[:], sig)
	if err != nil {
		return "", nil, err
	}

	wallet := crypto.PubkeyToAddress(*pubkey).Hex()

	return wallet, pubkey, nil
}
