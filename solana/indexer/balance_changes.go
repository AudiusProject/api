package indexer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"slices"

	"bridgerton.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processBalanceChanges(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	meta *rpc.TransactionMeta,
	tx *solana.Transaction,
	blockTime time.Time,
	txLogger *zap.Logger,
) error {
	trackedMints, err := getArtistCoins(ctx, db, false)
	if err != nil {
		return fmt.Errorf("failed to get artist coins: %w", err)
	}

	balanceChanges, err := extractBalanceChanges(meta, tx, trackedMints)
	if err != nil {
		return fmt.Errorf("failed to extract token balance changes: %w", err)
	}
	for acc, bal := range balanceChanges {
		row := balanceChangeRow{
			slot:           slot,
			account:        acc,
			balanceChange:  *bal,
			signature:      tx.Signatures[0].String(),
			blockTimestamp: blockTime,
		}
		err = insertBalanceChange(ctx, db, row)
		if err != nil {
			return fmt.Errorf("failed to insert balance change for account %s: %w", acc, err)
		}

		txLogger.Debug("balance change",
			zap.String("account", row.account),
			zap.String("mint", row.balanceChange.Mint),
			zap.Uint64("balance", row.balanceChange.PostTokenBalance),
			zap.Int64("change", row.balanceChange.Change),
		)
	}
	return nil
}

type balanceChange struct {
	Owner            string
	Mint             string
	PreTokenBalance  uint64
	PostTokenBalance uint64
	Change           int64
}

// Gets a map of account address to balance change from the given transaction.
func extractBalanceChanges(meta *rpc.TransactionMeta, tx *solana.Transaction, trackedMints []string) (map[string]*balanceChange, error) {
	balanceChanges := make(map[string]*balanceChange)

	// Make a list of all accounts involved in the transaction
	allAccounts, err := tx.Message.AccountMetaList()
	if err != nil {
		return nil, err
	}

	// Pre balances
	for _, balance := range meta.PreTokenBalances {
		isMintTracked := len(trackedMints) == 0 || slices.Contains(trackedMints, balance.Mint.String())
		if !isMintTracked {
			continue
		}

		acc := allAccounts[balance.AccountIndex].PublicKey
		preBalance, err := strconv.ParseUint(balance.UiTokenAmount.Amount, 10, 64)
		if err != nil {
			return balanceChanges, err
		}
		owner := ""
		if balance.Owner != nil {
			owner = balance.Owner.String()
		}

		balanceChanges[acc.String()] = &balanceChange{
			Owner:           owner,
			Mint:            balance.Mint.String(),
			PreTokenBalance: preBalance,
		}
	}

	// Post balances and changes
	for _, balance := range meta.PostTokenBalances {
		isMintTracked := len(trackedMints) == 0 || slices.Contains(trackedMints, balance.Mint.String())
		if !isMintTracked {
			continue
		}

		acc := allAccounts[balance.AccountIndex].PublicKey
		postBalance, err := strconv.ParseUint(balance.UiTokenAmount.Amount, 10, 64)
		if err != nil {
			return balanceChanges, err
		}

		b := balanceChanges[acc.String()]
		if b == nil {
			owner := ""
			if balance.Owner != nil {
				owner = balance.Owner.String()
			}
			b = &balanceChange{
				Owner:           owner,
				Mint:            balance.Mint.String(),
				PreTokenBalance: 0,
			}
			balanceChanges[acc.String()] = b
		}
		b.PostTokenBalance = postBalance
		b.Change = int64(b.PostTokenBalance) - int64(b.PreTokenBalance)
	}
	return balanceChanges, nil
}

type balanceChangeRow struct {
	balanceChange
	account        string
	signature      string
	slot           uint64
	blockTimestamp time.Time
}

func insertBalanceChange(ctx context.Context, db database.DBTX, row balanceChangeRow) error {
	sql := `INSERT INTO sol_token_account_balance_changes (owner, account, mint, change, balance, signature, slot, block_timestamp)
						VALUES (@owner, @account, @mint, @change, @balance, @signature, @slot, @blockTimestamp)
						ON CONFLICT DO NOTHING`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account":        row.account,
		"owner":          row.balanceChange.Owner,
		"mint":           row.balanceChange.Mint,
		"change":         row.balanceChange.Change,
		"balance":        row.balanceChange.PostTokenBalance,
		"signature":      row.signature,
		"slot":           row.slot,
		"blockTimestamp": row.blockTimestamp.UTC(),
	})
	return err
}
