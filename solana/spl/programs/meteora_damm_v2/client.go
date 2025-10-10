package meteora_damm_v2

import (
	"context"
	"math/big"
	"sort"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

type RpcClient interface {
	GetAccountDataBorshInto(ctx context.Context, account solana.PublicKey, out interface{}) error
}

type Client struct {
	client *rpc.Client
	logger *zap.Logger
}

func NewClient(
	client *rpc.Client,
	logger *zap.Logger,
) *Client {
	return &Client{
		client: client,
		logger: logger,
	}
}

// Gets the current Pool state.
func (c *Client) GetPool(ctx context.Context, account solana.PublicKey) (*Pool, error) {
	var pool Pool
	err := c.client.GetAccountDataBorshInto(ctx, account, &pool)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

// Gets a position by its address.
func (c *Client) GetPosition(ctx context.Context, account solana.PublicKey) (*PositionState, error) {
	var position PositionState
	err := c.client.GetAccountDataBorshInto(ctx, account, &position)
	if err != nil {
		return nil, err
	}
	return &position, nil
}

// Gets all position NFTs held by a wallet.
func (c *Client) GetPositionNFTs(ctx context.Context, owner solana.PublicKey) ([]solana.PublicKey, error) {
	accounts, err := c.client.GetTokenAccountsByOwner(ctx, owner, &rpc.GetTokenAccountsConfig{
		ProgramId: &solana.Token2022ProgramID,
	}, &rpc.GetTokenAccountsOpts{})
	if err != nil {
		return nil, err
	}

	var positionNFTs []solana.PublicKey
	for _, acc := range accounts.Value {
		data := token.Account{}
		bin.NewBorshDecoder(acc.Account.Data.GetBinary()).Decode(&data)
		if data.Amount == uint64(1) {
			positionNFTs = append(positionNFTs, data.Mint)
		}
	}

	return positionNFTs, nil
}

// Gets all the positions by an owner, sorted by total liquidity (descending).
func (c *Client) GetPositionsByOwner(ctx context.Context, owner solana.PublicKey) ([]*PositionState, error) {
	positionNFTs, err := c.GetPositionNFTs(ctx, owner)
	if err != nil {
		return nil, err
	}

	var positions []*PositionState
	for _, nft := range positionNFTs {
		pda, err := DerivePositionPDA(nft)
		if err != nil {
			return nil, err
		}
		position, err := c.GetPosition(ctx, pda)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}

	// Sort positions by total liquidity
	sort.Slice(positions, func(i, j int) bool {
		vestedLiquidity := (&big.Int{}).SetBytes(positions[i].VestedLiquidity.Bytes())
		permanentLockedLiquidity := (&big.Int{}).SetBytes(positions[i].PermanentLockedLiquidity.Bytes())
		unlockedLiquidity := (&big.Int{}).SetBytes(positions[i].UnlockedLiquidity.Bytes())
		totalLiquidityI := (&big.Int{}).Add(vestedLiquidity, permanentLockedLiquidity)
		totalLiquidityI.Add(totalLiquidityI, unlockedLiquidity)

		vestedLiquidity = (&big.Int{}).SetBytes(positions[j].VestedLiquidity.Bytes())
		permanentLockedLiquidity = (&big.Int{}).SetBytes(positions[j].PermanentLockedLiquidity.Bytes())
		unlockedLiquidity = (&big.Int{}).SetBytes(positions[j].UnlockedLiquidity.Bytes())
		totalLiquidityJ := (&big.Int{}).Add(vestedLiquidity, permanentLockedLiquidity)
		totalLiquidityJ.Add(totalLiquidityJ, unlockedLiquidity)

		return totalLiquidityJ.Cmp(totalLiquidityI) < 0
	})

	return positions, nil
}
