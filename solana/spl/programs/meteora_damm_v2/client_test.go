package meteora_damm_v2_test

import (
	"context"
	"testing"

	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestActualFetch(t *testing.T) {
	ctx := context.Background()
	client := meteora_damm_v2.NewClient(rpc.New(rpc.MainNetBeta_RPC), zap.NewNop())

	owner, err := solana.PublicKeyFromBase58("EF1zneAqA2mwjkD3Lj7sQnMhR2uorGqEHXNtAWfGdCu2")
	require.NoError(t, err)

	positions, err := client.GetPositionsByOwner(ctx, owner)
	require.NoError(t, err)
	require.Greater(t, len(positions), 0)
}
