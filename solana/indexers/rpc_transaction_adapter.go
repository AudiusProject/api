package indexers

import (
	"github.com/gagliardetto/solana-go/rpc"
)

// Implements the transactionAdapter interface for the Solana RPC client.
type rpcTransactionAdapter struct {
	tx *rpc.GetParsedTransactionResult
}

func (g *rpcTransactionAdapter) GetAllAccountKeys() []string {
	allAccounts := make([]string, len(g.tx.Transaction.Message.AccountKeys))
	for i, acc := range g.tx.Transaction.Message.AccountKeys {
		allAccounts[i] = acc.PublicKey.String()
	}
	return allAccounts
}

func (g *rpcTransactionAdapter) GetPreTokenBalances() []tokenBalanceAdapter {
	preBalances := make([]tokenBalanceAdapter, len(g.tx.Meta.PreTokenBalances))
	for i, balance := range g.tx.Meta.PreTokenBalances {
		preBalances[i] = &rpcTokenBalanceAdapter{
			tokenBalance: balance,
		}
	}
	return preBalances
}

func (g *rpcTransactionAdapter) GetPostTokenBalances() []tokenBalanceAdapter {
	preBalances := make([]tokenBalanceAdapter, len(g.tx.Meta.PostTokenBalances))
	for i, balance := range g.tx.Meta.PostTokenBalances {
		preBalances[i] = &rpcTokenBalanceAdapter{
			tokenBalance: balance,
		}
	}
	return preBalances
}

type rpcTokenBalanceAdapter struct {
	tokenBalance rpc.TokenBalance
}

func (g *rpcTokenBalanceAdapter) GetAccountIndex() uint32 {
	return uint32(g.tokenBalance.AccountIndex)
}
func (g *rpcTokenBalanceAdapter) GetMint() string {
	return g.tokenBalance.Mint.String()
}
func (g *rpcTokenBalanceAdapter) GetUiTokenAmount() uiTokenAmountAdapter {
	if g.tokenBalance.UiTokenAmount == nil {
		return nil
	}
	return &rpcUiTokenAmountAdapter{
		uiTokenAmount: g.tokenBalance.UiTokenAmount,
	}
}

func (g *rpcTokenBalanceAdapter) GetOwner() string {
	return g.tokenBalance.Owner.String()
}
func (g *rpcTokenBalanceAdapter) GetProgramId() string {
	return g.tokenBalance.ProgramId.String()
}

type rpcUiTokenAmountAdapter struct {
	uiTokenAmount *rpc.UiTokenAmount
}

func (g *rpcUiTokenAmountAdapter) GetAmount() string {
	return g.uiTokenAmount.Amount
}
func (g *rpcUiTokenAmountAdapter) GetDecimals() uint32 {
	return uint32(g.uiTokenAmount.Decimals)
}
func (g *rpcUiTokenAmountAdapter) GetUiAmount() float64 {
	if g.uiTokenAmount.UiAmount == nil {
		return 0
	}
	return *g.uiTokenAmount.UiAmount
}
func (g *rpcUiTokenAmountAdapter) GetUiAmountString() string {
	return g.uiTokenAmount.UiAmountString
}
