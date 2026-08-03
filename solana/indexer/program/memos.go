package program

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"
	"go.uber.org/zap"
)

// OLD_MEMO_PROGRAM_ID is the old memo program ID used for legacy memos.
var OLD_MEMO_PROGRAM_ID = solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

// Jupiter V6 router. When this program is in a claimable_tokens transfer's
// account_keys, the transfer is the "prepare" step of a swap-out withdrawal
// (user_bank USDC → some other token via Jupiter), regardless of memo.
var JUPITER_V6_PROGRAM_ID = solana.MustPublicKeyFromBase58("JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4")

// TransferType is the classification a claimable_tokens transfer or
// payment_router route gets based on the memos in its transaction.
type TransferType int

const (
	// TransferTypeUnknown means no recognized type memo was found — treat as
	// a bare transfer.
	TransferTypeUnknown TransferType = iota
	TransferTypeWithdrawal
	TransferTypePrepareWithdrawal
	TransferTypeInternalTransfer
	TransferTypeRecoverWithdrawal
)

// Memo string constants — these match the strings the legacy Python indexer
// (and the Audius web clients) write into the memo instruction at send time.
const (
	memoWithdrawal         = "Withdrawal"
	memoPrepareWithdrawal  = "Prepare Withdrawal"
	memoInternalTransfer   = "Internal Transfer"
	memoRecoverWithdrawal  = "Recover Withdrawal"
)

// parseTransferTypeMemo returns the TransferType that matches the memo's
// exact string, or TransferTypeUnknown if it doesn't match any known label.
func parseTransferTypeMemo(memo []byte) TransferType {
	switch string(memo) {
	case memoWithdrawal:
		return TransferTypeWithdrawal
	case memoPrepareWithdrawal:
		return TransferTypePrepareWithdrawal
	case memoInternalTransfer:
		return TransferTypeInternalTransfer
	case memoRecoverWithdrawal:
		return TransferTypeRecoverWithdrawal
	}
	return TransferTypeUnknown
}

// findTransferTypeMemo scans every memo instruction in the transaction and
// returns the first recognized TransferType. Unlike findNextPurchaseMemo it
// scans from the beginning — memos can land anywhere in the instruction list
// (e.g. before the secp256k1 signature or after the Transfer call), and
// there's at most one type-label memo per transaction in practice.
func findTransferTypeMemo(tx *solana.Transaction) TransferType {
	for _, inst := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			if t := parseTransferTypeMemo(inst.Data); t != TransferTypeUnknown {
				return t
			}
		}
	}
	return TransferTypeUnknown
}

// transactionTouchesJupiter reports whether the Jupiter V6 router appears in
// the transaction's account_keys (including LUT-resolved entries). Used by
// the claimable_tokens classifier to mark a transfer as "prepare withdrawal"
// even when the memo is missing.
func transactionTouchesJupiter(tx *solana.Transaction) bool {
	for _, key := range tx.Message.AccountKeys {
		if key.Equals(JUPITER_V6_PROGRAM_ID) {
			return true
		}
	}
	return false
}

type parsedPurchaseMemo struct {
	ContentType           string
	ContentId             int
	ValidAfterBlocknumber int
	BuyerUserId           int
	AccessType            string
}

func (m parsedPurchaseMemo) String() string {
	return fmt.Sprintf("%s:%d:%d:%d:%s", m.ContentType, m.ContentId, m.ValidAfterBlocknumber, m.BuyerUserId, m.AccessType)
}

func parsePurchaseMemo(memo []byte) (parsedPurchaseMemo, error) {
	parts := strings.Split(string(memo), ":")
	if len(parts) > 3 {
		contentType := parts[0]

		contentId, err := strconv.Atoi(parts[1])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse contentId: %w", err)
		}

		validAfterBlocknumber, err := strconv.Atoi(parts[2])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse validAfterBlocknumber: %w", err)
		}

		buyerUserId, err := strconv.Atoi(parts[3])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse buyerUserId: %w", err)
		}

		accessType := "stream"
		if len(parts) > 4 {
			accessType = parts[4]
		}
		parsed := parsedPurchaseMemo{
			ContentType:           contentType,
			ContentId:             contentId,
			ValidAfterBlocknumber: validAfterBlocknumber,
			BuyerUserId:           buyerUserId,
			AccessType:            accessType,
		}
		return parsed, nil
	}
	return parsedPurchaseMemo{}, errors.New("not a purchase memo")
}

func findNextPurchaseMemo(tx *solana.Transaction, instructionIndex int, logger *zap.Logger) (parsedPurchaseMemo, bool) {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			parsed, err := parsePurchaseMemo(inst.Data)
			if err != nil {
				if logger != nil {
					logger.Warn("failed to parse purchase memo", zap.Error(err), zap.String("memo", string(inst.Data)))
				}
				continue
			}
			return parsed, true
		}
	}
	return parsedPurchaseMemo{}, false
}

type parsedLocationMemo struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

func parseLocationMemo(memo []byte) (parsedLocationMemo, error) {
	if len(memo) > 3 && string(memo[0:3]) == "geo" {
		var parsed parsedLocationMemo
		err := json.Unmarshal(memo[4:], &parsed)
		if err != nil {
			return parsedLocationMemo{}, err
		}
		return parsed, nil
	}
	return parsedLocationMemo{}, errors.New("not a location memo")
}

func findNextLocationMemo(tx *solana.Transaction, instructionIndex int, logger *zap.Logger) parsedLocationMemo {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			parsed, err := parseLocationMemo(inst.Data)
			if err != nil {
				if logger != nil {
					logger.Warn("failed to parse location memo", zap.Error(err), zap.String("memo", string(inst.Data)))
				}
				continue
			}
			return parsed
		}
	}
	return parsedLocationMemo{}
}
