package dbv1

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type GetUserAudioTransactionsParams struct {
	UserID int32
	// TODO: enums?
	SortDirection string
	SortMethod    string
	OffsetVal     int32
	LimitVal      int32
}

// UserAudioTransaction represents a common type for audio transaction rows
// that can be used for both date and type sorted queries
type UserAudioTransaction struct {
	CreatedAt       time.Time   `json:"transaction_date"`
	TransactionType string      `json:"transaction_type"`
	Method          string      `json:"method"`
	UserBank        string      `json:"user_bank"`
	TxMetadata      pgtype.Text `json:"metadata"`
	Signature       string      `json:"signature"`
	Change          string      `json:"change"`
	Balance         string      `json:"balance"`
}

func (r GetUserAudioTransactionsSortedByDateRow) ToUserAudioTransaction() UserAudioTransaction {
	return UserAudioTransaction{
		CreatedAt:       r.CreatedAt,
		TransactionType: r.TransactionType,
		Method:          r.Method,
		UserBank:        r.UserBank,
		TxMetadata:      r.TxMetadata,
		Signature:       r.Signature,
		Change:          r.Change,
		Balance:         r.Balance,
	}
}

func (r GetUserAudioTransactionsSortedByTypeRow) ToUserAudioTransaction() UserAudioTransaction {
	return UserAudioTransaction{
		CreatedAt:       r.CreatedAt,
		TransactionType: r.TransactionType,
		Method:          r.Method,
		UserBank:        r.UserBank,
		TxMetadata:      r.TxMetadata,
		Signature:       r.Signature,
		Change:          r.Change,
		Balance:         r.Balance,
	}
}

func (q *Queries) GetUserAudioTransactions(ctx context.Context, arg GetUserAudioTransactionsParams) ([]UserAudioTransaction, error) {
	if arg.SortMethod == "date" {
		rows, err := q.GetUserAudioTransactionsSortedByDate(ctx, GetUserAudioTransactionsSortedByDateParams{
			UserID:        arg.UserID,
			SortDirection: arg.SortDirection,
			OffsetVal:     arg.OffsetVal,
			LimitVal:      arg.LimitVal,
		})
		if err != nil {
			return nil, err
		}
		userAudioTransactions := make([]UserAudioTransaction, len(rows))
		for i, row := range rows {
			userAudioTransactions[i] = row.ToUserAudioTransaction()
		}
		return userAudioTransactions, nil
	} else if arg.SortMethod == "type" {
		rows, err := q.GetUserAudioTransactionsSortedByType(ctx, GetUserAudioTransactionsSortedByTypeParams{
			UserID:        arg.UserID,
			SortDirection: arg.SortDirection,
			OffsetVal:     arg.OffsetVal,
			LimitVal:      arg.LimitVal,
		})
		if err != nil {
			return nil, err
		}
		userAudioTransactions := make([]UserAudioTransaction, len(rows))
		for i, row := range rows {
			userAudioTransactions[i] = row.ToUserAudioTransaction()
		}
		return userAudioTransactions, nil
	}
	// TODO: should be unnecessary if we use an enum
	return nil, errors.New("invalid sort method")
}
