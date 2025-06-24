package indexers

import (
	"github.com/mr-tron/base58"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// Implements the transactionAdapter interface for the Geyser gRPC client.
type geyserTransactionAdapter struct {
	tx *pb.SubscribeUpdateTransactionInfo
}

func (g *geyserTransactionAdapter) GetAllAccountKeys() []string {
	allAccounts := make(
		[]string,
		len(g.tx.Transaction.Message.AccountKeys)+
			len(g.tx.Meta.LoadedWritableAddresses)+
			len(g.tx.Meta.LoadedReadonlyAddresses),
	)
	for i, acc := range g.tx.Transaction.Message.AccountKeys {
		allAccounts[i] = base58.Encode(acc)
	}
	for i, acc := range g.tx.Meta.LoadedWritableAddresses {
		allAccounts[len(g.tx.Transaction.Message.AccountKeys)+i] =
			base58.Encode(acc)
	}
	for i, acc := range g.tx.Meta.LoadedReadonlyAddresses {
		allAccounts[len(g.tx.Transaction.Message.AccountKeys)+
			len(g.tx.Meta.LoadedWritableAddresses)+
			i] = base58.Encode(acc)
	}
	return allAccounts
}

func (g *geyserTransactionAdapter) GetPreTokenBalances() []tokenBalanceAdapter {
	preBalances := make([]tokenBalanceAdapter, len(g.tx.Meta.PreTokenBalances))
	for i, balance := range g.tx.Meta.PreTokenBalances {
		preBalances[i] = &geyserTokenBalanceAdapter{
			tokenBalance: balance,
		}
	}
	return preBalances
}

func (g *geyserTransactionAdapter) GetPostTokenBalances() []tokenBalanceAdapter {
	preBalances := make([]tokenBalanceAdapter, len(g.tx.Meta.PostTokenBalances))
	for i, balance := range g.tx.Meta.PostTokenBalances {
		preBalances[i] = &geyserTokenBalanceAdapter{
			tokenBalance: balance,
		}
	}
	return preBalances
}

type geyserTokenBalanceAdapter struct {
	tokenBalance *pb.TokenBalance
}

func (g *geyserTokenBalanceAdapter) GetAccountIndex() uint32 {
	return g.tokenBalance.AccountIndex
}
func (g *geyserTokenBalanceAdapter) GetMint() string {
	return g.tokenBalance.Mint
}
func (g *geyserTokenBalanceAdapter) GetUiTokenAmount() uiTokenAmountAdapter {
	if g.tokenBalance.UiTokenAmount == nil {
		return nil
	}
	return &geyserUiTokenAmountAdapter{
		uiTokenAmount: g.tokenBalance.UiTokenAmount,
	}
}

func (g *geyserTokenBalanceAdapter) GetOwner() string {
	return g.tokenBalance.Owner
}
func (g *geyserTokenBalanceAdapter) GetProgramId() string {
	return g.tokenBalance.ProgramId
}

type geyserUiTokenAmountAdapter struct {
	uiTokenAmount *pb.UiTokenAmount
}

func (g *geyserUiTokenAmountAdapter) GetAmount() string {
	return g.uiTokenAmount.Amount
}
func (g *geyserUiTokenAmountAdapter) GetDecimals() uint32 {
	return g.uiTokenAmount.Decimals
}
func (g *geyserUiTokenAmountAdapter) GetUiAmount() float64 {
	return g.uiTokenAmount.UiAmount
}
func (g *geyserUiTokenAmountAdapter) GetUiAmountString() string {
	return g.uiTokenAmount.UiAmountString
}
