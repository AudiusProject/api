package indexers

// Defines the interface transactions from different sources
// (like RPC or Geyser) must implement to provide a unified way to access
// transaction data.
type transactionAdapter interface {
	GetAllAccountKeys() []string
	GetPreTokenBalances() []tokenBalanceAdapter
	GetPostTokenBalances() []tokenBalanceAdapter
}

type tokenBalanceAdapter interface {
	GetAccountIndex() uint32
	GetMint() string
	GetUiTokenAmount() uiTokenAmountAdapter
	GetOwner() string
	GetProgramId() string
}

type uiTokenAmountAdapter interface {
	GetAmount() string
	GetDecimals() uint32
	GetUiAmount() float64
	GetUiAmountString() string
}
