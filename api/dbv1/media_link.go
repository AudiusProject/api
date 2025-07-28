package dbv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/rendezvous"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type MediaLink struct {
	Url     string   `json:"url"`
	Mirrors []string `json:"mirrors"`
}

type Id3Tags struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

func mediaLink(cid string, trackId int32, userId int32, id3Tags *Id3Tags) (*MediaLink, error) {
	first, rest := rendezvous.GlobalHasher.ReplicaSet3(cid)

	timestamp := time.Now().Unix() * 1000
	data := map[string]interface{}{
		"cid":       cid,
		"timestamp": timestamp,
		"trackId":   trackId,
		"userId":    userId,
	}

	signature, err := generateSignature(data)
	if err != nil {
		return nil, err
	}

	// Convert the data map to a JSON string
	dataJSON, _ := json.Marshal(data)

	signatureData := map[string]interface{}{
		"signature": signature,
		"data":      string(dataJSON),
	}
	signatureJSON, _ := json.Marshal(signatureData)
	queryParams := url.Values{}
	queryParams.Set("signature", string(signatureJSON))

	basePath := fmt.Sprintf("tracks/cidstream/%s", cid)
	path := fmt.Sprintf("%s?%s", basePath, queryParams.Encode())

	if id3Tags != nil {
		queryParams.Set("id3", "true")
		queryParams.Set("id3_artist", id3Tags.Artist)
		queryParams.Set("id3_title", id3Tags.Title)
	}

	return &MediaLink{
		Url:     fmt.Sprintf("%s/%s", first, path),
		Mirrors: rest,
	}, nil
}

func generateSignature(data map[string]interface{}) (string, error) {
	ecdsaPrivKey, err := crypto.HexToECDSA(config.Cfg.DelegatePrivateKey)
	if err != nil {
		return "", err
	}

	// Sort json
	jsonStr := func(data map[string]interface{}) string {
		var b bytes.Buffer
		_ = json.NewEncoder(&b).Encode(data)
		return strings.TrimRight(b.String(), "\n")
	}(data)

	// Hash the JSON string, prefix it, and hash again
	messageHash := crypto.Keccak256([]byte(jsonStr))
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(messageHash))
	prefixedMessage := append([]byte(prefix), messageHash...)
	finalHash := crypto.Keccak256(prefixedMessage)

	// Sign the hash with the private key
	signature, err := crypto.Sign(finalHash, ecdsaPrivKey)
	if err != nil {
		return "", err
	}

	return hexutil.Encode(signature), nil
}
