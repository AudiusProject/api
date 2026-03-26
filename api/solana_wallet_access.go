package api

import (
	"context"
	"math"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

// checkSolanaWalletTokenAccess checks whether a Solana wallet holds sufficient tokens
// for a coin-gated track by doing a real-time on-chain balance lookup.
func (app *ApiServer) checkSolanaWalletTokenAccess(
	ctx context.Context,
	solanaWallet string,
	tokenMint string,
	requiredAmount int64,
) (bool, error) {
	walletPubkey, err := solana.PublicKeyFromBase58(solanaWallet)
	if err != nil {
		return false, err
	}

	mintPubkey, err := solana.PublicKeyFromBase58(tokenMint)
	if err != nil {
		return false, err
	}

	// Derive the associated token account address
	ata, _, err := solana.FindAssociatedTokenAddress(walletPubkey, mintPubkey)
	if err != nil {
		return false, err
	}

	// Get token balance from chain
	balanceResult, err := app.solanaRpcClient.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
	if err != nil {
		// Account doesn't exist means zero balance
		app.logger.Debug("checkSolanaWalletTokenAccess: token account not found",
			zap.String("wallet", solanaWallet),
			zap.String("mint", tokenMint),
			zap.Error(err),
		)
		return false, nil
	}

	if balanceResult == nil || balanceResult.Value == nil {
		return false, nil
	}

	rawBalance := balanceResult.Value.Amount
	if rawBalance == "" {
		return false, nil
	}

	// Parse the raw balance (string of lamports/smallest unit)
	var balance uint64
	for _, c := range rawBalance {
		if c < '0' || c > '9' {
			return false, nil
		}
		balance = balance*10 + uint64(c-'0')
	}

	// Look up coin decimals from the artist_coins table
	var decimals int32
	err = app.pool.QueryRow(ctx, `
		SELECT decimals FROM artist_coins WHERE mint = $1
	`, tokenMint).Scan(&decimals)
	if err != nil {
		// If coin not found, try using the decimals from the RPC response
		decimals = int32(balanceResult.Value.Decimals)
	}

	// Scale required amount by decimals
	scaledRequired := requiredAmount * int64(math.Pow10(int(decimals)))

	app.logger.Debug("checkSolanaWalletTokenAccess",
		zap.String("wallet", solanaWallet),
		zap.String("mint", tokenMint),
		zap.Uint64("balance", balance),
		zap.Int64("scaledRequired", scaledRequired),
		zap.Bool("hasAccess", int64(balance) >= scaledRequired),
	)

	return int64(balance) >= scaledRequired, nil
}
