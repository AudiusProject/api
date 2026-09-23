package dbv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
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
	first, rest := rendezvous.GlobalHasher.Select(cid)

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

	if id3Tags != nil {
		queryParams.Set("id3", "true")
		queryParams.Set("id3_artist", id3Tags.Artist)
		queryParams.Set("id3_title", id3Tags.Title)
	}

	basePath := fmt.Sprintf("tracks/cidstream/%s", cid)
	path := fmt.Sprintf("%s?%s", basePath, queryParams.Encode())

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

// DownloadCid is the blob a download serves: the artist's original upload when
// the row kept one, otherwise the transcode. Empty when the row has neither,
// which happens on uploads whose cid backfill never ran - signing an empty cid
// only produces a content-node URL guaranteed to 404.
func (t *GetTracksRow) DownloadCid() string {
	if t.OrigFileCid.String != "" {
		return t.OrigFileCid.String
	}
	return t.TrackCid.String
}

// SignDownloadLink signs a download URL for this track without consulting
// is_downloadable or the download gates. Nothing here re-checks access, so the
// caller must already have established that the requester may bypass them -
// today only a wallet-verified owner or account manager does, see
// ApiServer.ownerDownloadLink. Returns nil when the row has no cid to sign.
func (t *Track) SignDownloadLink(userId int32) (*MediaLink, error) {
	cid := t.DownloadCid()
	if cid == "" {
		return nil, nil
	}
	return mediaLink(cid, t.TrackID, userId, nil)
}
