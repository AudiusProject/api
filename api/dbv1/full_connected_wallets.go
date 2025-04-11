package dbv1

import "context"

type FullConnectedWallets struct {
	ErcWallets []string `json:"erc_wallets"`
	SplWallets []string `json:"spl_wallets"`
}

func (q *Queries) FullConnectedWallets(ctx context.Context, userId int32) (*FullConnectedWallets, error) {
	rows, err := q.GetUserConnectedWallets(ctx, userId)
	if err != nil {
		return nil, err
	}

	fullConnectedWallets := FullConnectedWallets{}
	for _, row := range rows {
		if row.Chain == WalletChainEth {
			fullConnectedWallets.ErcWallets = append(fullConnectedWallets.ErcWallets, row.Wallet)
		}
		if row.Chain == WalletChainSol {
			fullConnectedWallets.SplWallets = append(fullConnectedWallets.SplWallets, row.Wallet)
		}
	}

	return &fullConnectedWallets, nil
}
