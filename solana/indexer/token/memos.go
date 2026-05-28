package token

import (
	"context"
	"strings"

	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
)

// OLD_MEMO_PROGRAM_ID is the legacy memo program ID; some clients still
// emit memos under this one alongside the canonical solana.MemoProgramID.
var oldMemoProgramID = solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

// Vendor map for AUDIO top-up memos written by the web client / wAUDIO
// Stripe-Coinbase rails (mirrors the Python index_spl_token purchase_vendor_map).
// Memo format: "In-App $AUDIO Purchase: <Vendor>"
const audioPurchaseMemoPrefix = "In-App $AUDIO Purchase: "

var audioPurchaseVendorMemoTypes = map[string]string{
	"Link by Stripe": "purchase_stripe",
	"Coinbase Pay":   "purchase_coinbase",
	"Unknown":        "purchase_unknown",
}

// findAudioVendorMemoType scans every memo instruction in the transaction for
// the AUDIO vendor purchase prefix and returns the matching memo_type string
// (purchase_stripe / purchase_coinbase / purchase_unknown) along with the
// instruction index where the memo was found. Returns ("", -1, false) if no
// recognized vendor memo is present.
func findAudioVendorMemoType(tx *solana.Transaction) (memoType string, instructionIndex int, ok bool) {
	for i, inst := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if !programId.Equals(solana.MemoProgramID) && !programId.Equals(oldMemoProgramID) {
			continue
		}
		data := string(inst.Data)
		if !strings.HasPrefix(data, audioPurchaseMemoPrefix) {
			continue
		}
		vendor := strings.TrimPrefix(data, audioPurchaseMemoPrefix)
		if t, found := audioPurchaseVendorMemoTypes[vendor]; found {
			return t, i, true
		}
	}
	return "", -1, false
}

// insertAudioVendorMemoMarker writes a row into sol_transfer_memo_types so
// v_token_transactions_history classifies the balance change as the right
// purchase_* type instead of a bare transfer. Same destination table as the
// claimable-tokens / payment-router memo markers — the view reads memo_type
// by signature, so a single (signature, instruction_index) row is enough.
func insertAudioVendorMemoMarker(ctx context.Context, db database.DBTX, signature string, instructionIndex int, slot uint64, memoType string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO sol_transfer_memo_types (signature, instruction_index, slot, memo_type)
		VALUES (@signature, @instructionIndex, @slot, @memoType)
		ON CONFLICT DO NOTHING
	;`, pgx.NamedArgs{
		"signature":        signature,
		"instructionIndex": instructionIndex,
		"slot":             slot,
		"memoType":         memoType,
	})
	return err
}
