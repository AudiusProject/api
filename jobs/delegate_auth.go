package jobs

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// signedHTTPGet performs an HTTP GET signed with a delegate private key.
// Mirrors apps' src/utils/auth_helpers.py:signed_get():
//
//	timestamp = round(time.time() * 1000)
//	signature = sign(timestamp, private_key)
//	nonce = f"{timestamp}:{signature.hex()}"
//	Authorization: Basic base64(nonce)
//
//	where sign() does:
//	    digest = keccak(timestamp_text)
//	    signature = ETH-personal-sign(digest_hex, private_key)
//
// `delegatePrivateKey` is the 0x-prefixed (or unprefixed) hex string from
// config.Cfg.DelegatePrivateKey.
func signedHTTPGet(client *http.Client, url, delegatePrivateKey string) (*http.Response, error) {
	auth, err := basicAuthNonce(delegatePrivateKey, time.Now())
	if err != nil {
		return nil, fmt.Errorf("build auth: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)

	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// drainResponse closes the body of a non-2xx response and returns an error
// including the status + first 512 bytes of body for context.
func drainResponse(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// basicAuthNonce returns the Authorization header value used by apps'
// signed_get. Exported through signedHTTPGet but extracted for testing.
func basicAuthNonce(delegatePrivateKey string, now time.Time) (string, error) {
	pkHex := strings.TrimPrefix(delegatePrivateKey, "0x")
	pkBytes, err := hex.DecodeString(pkHex)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	pk, err := crypto.ToECDSA(pkBytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	// Python: timestamp = round(time.time() * 1000)
	tsStr := fmt.Sprintf("%d", now.UnixMilli())

	// Python: digest = Web3.keccak(text=timestamp_str).hex()
	digestBytes := crypto.Keccak256([]byte(tsStr))

	// Python: encode_defunct(hexstr=digest_hex) -> EIP-191 personal-sign envelope
	// over the BYTES that digest_hex decodes to (i.e. the raw keccak digest).
	// We replicate that here.
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(digestBytes))
	personalDigest := crypto.Keccak256(append([]byte(prefix), digestBytes...))

	sig, err := crypto.Sign(personalDigest, pk)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	// go-ethereum's crypto.Sign returns recovery id 0 or 1; web3.py returns
	// 27 or 28. Normalize to match apps.
	if sig[64] < 27 {
		sig[64] += 27
	}

	// The signature hex MUST carry the "0x" prefix. apps builds the nonce as
	// f"{timestamp}:{signature.hex()}" where signature is a web3.py HexBytes;
	// under hexbytes 0.3.1 (apps' pinned version) HexBytes.hex() returns the
	// string WITH a leading "0x". The notifier's verifier parses the sig
	// assuming that prefix (it strips the first two chars). Without "0x" the
	// whole 65-byte signature shifts by one byte: r/s are corrupted and the
	// recovery byte is read from the wrong offset — yielding a value outside
	// [0,3] and the notifier's "recovery param is more than two bits" 401.
	// mediorum's basic_auth.go does the same `0x`-prefixing for this reason.
	nonce := tsStr + ":0x" + hex.EncodeToString(sig)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(nonce)), nil
}
