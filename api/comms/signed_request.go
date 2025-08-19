package comms

import (
	"encoding/base64"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
)

func ReadSignedPost(c *fiber.Ctx) ([]byte, string, error) {
	if c.Method() != "POST" {
		return nil, "", errors.New("readSignedPost bad method: " + c.Method())
	}

	payload := c.Body()

	sigHex := c.Get(SigHeader)
	wallet, err := recoverSigningWallet(sigHex, payload)
	return payload, wallet, err
}

func recoverSigningWallet(signatureHex string, signedData []byte) (string, error) {
	sig, err := base64.StdEncoding.DecodeString(signatureHex)
	if err != nil {
		err = errors.New("bad sig header: " + err.Error())
		return "", err
	}

	// recover
	hash := crypto.Keccak256Hash(signedData)
	pubkey, err := crypto.SigToPub(hash[:], sig)
	if err != nil {
		return "", err
	}

	wallet := crypto.PubkeyToAddress(*pubkey).Hex()

	// TODO: Still need this? We have a function for getting these in another file
	// seed the user pubkey if missing
	// err = pubkeystore.SetPubkeyForWallet(wallet, pubkey)
	// if err != nil {
	// 	slog.Warn("failed to SetPubkeyForWallet", "wallet", wallet, "err", err)
	// } else {
	// 	slog.Info("SetPubkeyForWallet OK", "wallet", wallet)
	// }

	return wallet, nil
}
