package dbv1

import (
	"testing"

	"bridgerton.audius.co/config"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSignature(t *testing.T) {
	originalCfg := config.Cfg
	config.Cfg = config.Config{
		// Dummy key
		DelegatePrivateKey: "0633fddb74e32b3cbc64382e405146319c11a1a52dc96598e557c5dbe2f31468",
	}
	defer func() {
		config.Cfg = originalCfg
	}()

	data := map[string]interface{}{
		"cid":       "baeaaaiqsecjitceegveqtb67yhksnwe75w4khfsep5obuuljl2il3wwnm22su",
		"timestamp": int64(1744657599000),
		"trackId":   int32(1462669012),
		"userId":    int32(0),
	}

	sig, err := generateSignature(data)
	assert.NoError(t, err)
	assert.Equal(t, "0xc02c72af125318dcb85eaf8edf6499bec3b17a91e0153b4d89a97cca661746291dde3d06b6b358d77df046eb9e60d65ab1d2a2e4579ae745874186be03957dbc00", sig)
}

func TestGenerateSignatureBadPrivateKey(t *testing.T) {
	originalCfg := config.Cfg
	config.Cfg = config.Config{
		DelegatePrivateKey: "bad-private-key",
	}
	defer func() {
		config.Cfg = originalCfg
	}()

	data := map[string]interface{}{
		"cid":       "baeaaaiqsecjitceegveqtb67yhksnwe75w4khfsep5obuuljl2il3wwnm22su",
		"timestamp": int64(1744657599000),
		"trackId":   int32(1462669012),
		"userId":    int32(0),
	}

	_, err := generateSignature(data)
	assert.Error(t, err)
}
