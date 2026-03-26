package api

import (
	"context"

	"api.audius.co/api/dbv1"
)

// newSolanaWalletTokenBalanceFetcher returns a TokenBalanceFetcher that looks up
// on-chain token balances for the given Solana wallet from the indexed
// sol_token_account_balances table. The returned balances are raw (smallest unit)
// matching the format used by GetBulkTrackAccess.
func (app *ApiServer) newSolanaWalletTokenBalanceFetcher(wallet string) dbv1.TokenBalanceFetcher {
	return func(ctx context.Context, mints []string) (map[string]int64, error) {
		balances := make(map[string]int64, len(mints))
		rows, err := app.pool.Query(ctx, `
			SELECT mint, COALESCE(balance, 0)
			FROM sol_token_account_balances
			WHERE owner = $1
			AND mint = ANY($2)
		`, wallet, mints)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var mint string
			var balance int64
			if err := rows.Scan(&mint, &balance); err == nil {
				balances[mint] = balance
			}
		}
		return balances, rows.Err()
	}
}
