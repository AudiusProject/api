package token

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/assert"
)

// makeMemoTx builds a minimal solana.Transaction whose instructions are a
// list of memo-program payloads in order. Anything in `payloads` becomes a
// memo instruction; non-memo instructions are not modelled (findAudioVendorMemoType
// only cares about memo-program entries anyway).
func makeMemoTx(payloads ...string) *solana.Transaction {
	tx := &solana.Transaction{}
	tx.Message.AccountKeys = solana.PublicKeySlice{solana.MemoProgramID}
	for _, p := range payloads {
		tx.Message.Instructions = append(tx.Message.Instructions, solana.CompiledInstruction{
			ProgramIDIndex: 0,
			Data:           []byte(p),
		})
	}
	return tx
}

func TestFindAudioVendorMemoType(t *testing.T) {
	cases := []struct {
		name     string
		payloads []string
		wantType string
		wantOk   bool
	}{
		{
			name:     "stripe",
			payloads: []string{"In-App $AUDIO Purchase: Link by Stripe"},
			wantType: "purchase_stripe",
			wantOk:   true,
		},
		{
			name:     "coinbase",
			payloads: []string{"In-App $AUDIO Purchase: Coinbase Pay"},
			wantType: "purchase_coinbase",
			wantOk:   true,
		},
		{
			name:     "unknown",
			payloads: []string{"In-App $AUDIO Purchase: Unknown"},
			wantType: "purchase_unknown",
			wantOk:   true,
		},
		{
			name:     "unrecognized vendor name skipped",
			payloads: []string{"In-App $AUDIO Purchase: Apple Pay"},
			wantOk:   false,
		},
		{
			name:     "non-vendor memo skipped",
			payloads: []string{"Withdrawal"},
			wantOk:   false,
		},
		{
			name:     "empty tx",
			payloads: nil,
			wantOk:   false,
		},
		{
			name:     "picks first matching vendor when multiple memos present",
			payloads: []string{"some other memo", "In-App $AUDIO Purchase: Link by Stripe"},
			wantType: "purchase_stripe",
			wantOk:   true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotType, gotIdx, gotOk := findAudioVendorMemoType(makeMemoTx(c.payloads...))
			assert.Equal(t, c.wantOk, gotOk)
			if c.wantOk {
				assert.Equal(t, c.wantType, gotType)
				assert.True(t, gotIdx >= 0)
			}
		})
	}
}
