package launchpad

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"

	"api.audius.co/database"
	"api.audius.co/utils"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A mint launched under some earlier launchpad secret. Its Solana reward
// manager account was created at launch and cannot move, so once the secret is
// rotated the configured secret derives a different reward manager — one with
// no Solana account. Reward-code creation used to bind rewards to that derived
// value and build a parallel pool against it, silently.
//
// Synthetic values; the behavior does not depend on which pubkeys these are.
const (
	launchedMint = "GkQ4dGqXk1sTVpsxWpsLmWEDRHzHHKtVdiHnRPMcbTBd"
	// The reward manager that actually exists on Solana for launchedMint,
	// as indexed from its InitRewardManager instruction.
	onChainRewardManager = "HRRe6fbSDudpsBmkfBnLNHQnKkKgvhVc4pdBfR9U1YQz"
)

// rotatedSecret stands in for a launchpad secret that is not the one the mint
// was launched under, so the reward manager it derives is not the mint's.
const rotatedSecret = "0011223344556677889900112233445566778899001122334455667788990011"

// fakeRewardPoolClient records what the caller asked cometbft to do so a test
// can assert that no pool was created, which is the whole point: the
// production failure was silent precisely because a pool got created.
type fakeRewardPoolClient struct {
	pools        map[string]bool
	getCalls     []string
	createCalls  []*v1.CreateRewardPool
	createdRmKey []ed25519.PrivateKey
}

func newFakeRewardPoolClient(existingPools ...string) *fakeRewardPoolClient {
	pools := map[string]bool{}
	for _, p := range existingPools {
		pools[p] = true
	}
	return &fakeRewardPoolClient{pools: pools}
}

func (f *fakeRewardPoolClient) GetRewardPool(ctx context.Context, rewardsManagerPubkey string) (*v1.GetRewardPoolResponse, error) {
	f.getCalls = append(f.getCalls, rewardsManagerPubkey)
	if f.pools[rewardsManagerPubkey] {
		return &v1.GetRewardPoolResponse{}, nil
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("reward pool not found"))
}

func (f *fakeRewardPoolClient) CreateRewardPool(ctx context.Context, msg *v1.CreateRewardPool, rmKey ed25519.PrivateKey, deadlineBlockHeight int64) (string, error) {
	f.createCalls = append(f.createCalls, msg)
	f.createdRmKey = append(f.createdRmKey, rmKey)
	f.pools[msg.RewardsManagerPubkey] = true
	return "txhash", nil
}

func (f *fakeRewardPoolClient) createdPubkeys() []string {
	out := []string{}
	for _, c := range f.createCalls {
		out = append(out, c.RewardsManagerPubkey)
	}
	return out
}

func seedRewardManagerInit(t *testing.T, pool *pgxpool.Pool, mint, rewardManagerState string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO sol_reward_manager_inits
			(signature, instruction_index, slot, min_votes, reward_manager_state, token_source, mint, manager, authority)
		VALUES ($1, 0, 100, 3, $2, 'tokenSource', $3, 'manager', 'authority')
	`, "sig-"+mint, rewardManagerState, mint)
	require.NoError(t, err)
}

func derivedRewardManager(secret string, mint solana.PublicKey) string {
	key := utils.DeriveRewardManagerKeypair(secret, mint)
	return base58.Encode(key.Public().(ed25519.PublicKey))
}

func TestPrepareRewardPool(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	ctx := context.Background()
	logger := zap.NewNop()

	mint := solana.MustPublicKeyFromBase58(launchedMint)

	// Sanity: the scenario under test is only meaningful if the configured
	// secret derives something other than the mint's real reward manager.
	require.NotEqual(t, onChainRewardManager, derivedRewardManager(rotatedSecret, mint))

	t.Run("launched mint after a secret rotation reuses its real pool and creates nothing", func(t *testing.T) {
		seedRewardManagerInit(t, pool, launchedMint, onChainRewardManager)
		// cometbft already has the pool for the mint's real reward manager
		// (441 rewards' worth of history, in production).
		client := newFakeRewardPoolClient(onChainRewardManager)

		rm, err := PrepareRewardPool(ctx, logger, pool, client, rotatedSecret, mint, "0xclaimauthority", 1000)
		require.NoError(t, err)

		assert.Equal(t, onChainRewardManager, rm,
			"rewards must bind to the reward manager that exists on Solana, not one derived from the current secret")
		assert.Empty(t, client.createdPubkeys(),
			"an established mint must never look brand-new; creating a pool here is the phantom-pool bug")
		assert.Equal(t, []string{onChainRewardManager}, client.getCalls)
	})

	t.Run("rotated secret with no existing pool fails loudly instead of creating a phantom pool", func(t *testing.T) {
		rotatedMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
		otherRewardManager := "HRRe6fbSDudpsBmkfBnLNHQnKkKgvhVc4pdBfR9U1YQy"
		seedRewardManagerInit(t, pool, rotatedMint.String(), otherRewardManager)
		client := newFakeRewardPoolClient()

		_, err := PrepareRewardPool(ctx, logger, pool, client, rotatedSecret, rotatedMint, "0xclaimauthority", 1000)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRewardManagerMismatch)
		assert.Contains(t, err.Error(), otherRewardManager)
		assert.Contains(t, err.Error(), derivedRewardManager(rotatedSecret, rotatedMint))
		assert.Empty(t, client.createdPubkeys(),
			"a pool signed by a key that has no Solana reward manager account must never be created")
	})

	t.Run("mint with no indexed reward manager fails without touching cometbft", func(t *testing.T) {
		unlaunched := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
		client := newFakeRewardPoolClient()

		_, err := PrepareRewardPool(ctx, logger, pool, client, rotatedSecret, unlaunched, "0xclaimauthority", 1000)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRewardManagerNotIndexed)
		assert.Empty(t, client.getCalls)
		assert.Empty(t, client.createdPubkeys())
	})

	t.Run("mint launched under the current secret still creates its pool", func(t *testing.T) {
		newMint := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		realRewardManager := derivedRewardManager(rotatedSecret, newMint)
		seedRewardManagerInit(t, pool, newMint.String(), realRewardManager)
		client := newFakeRewardPoolClient()

		rm, err := PrepareRewardPool(ctx, logger, pool, client, rotatedSecret, newMint, "0xclaimauthority", 1000)
		require.NoError(t, err)

		assert.Equal(t, realRewardManager, rm)
		assert.Equal(t, []string{realRewardManager}, client.createdPubkeys())
		assert.Equal(t, []string{"0xclaimauthority"}, client.createCalls[0].Authorities)
		// The pool must be signed by the private half of the reward manager
		// that Solana has, which is what makes rm_owner_signature verify.
		require.Len(t, client.createdRmKey, 1)
		assert.Equal(t, realRewardManager, base58.Encode(client.createdRmKey[0].Public().(ed25519.PublicKey)))
	})

	t.Run("existing pool for a matching derivation is reused", func(t *testing.T) {
		newMint := solana.MustPublicKeyFromBase58("mSoLzYCxHdYgdzU16g5QSh3i5K3z3KZK7ytfqcJm7So")
		realRewardManager := derivedRewardManager(rotatedSecret, newMint)
		seedRewardManagerInit(t, pool, newMint.String(), realRewardManager)
		client := newFakeRewardPoolClient(realRewardManager)

		rm, err := PrepareRewardPool(ctx, logger, pool, client, rotatedSecret, newMint, "0xclaimauthority", 1000)
		require.NoError(t, err)

		assert.Equal(t, realRewardManager, rm)
		assert.Empty(t, client.createdPubkeys())
	})
}

func TestResolveRewardManager(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_api")
	ctx := context.Background()

	t.Run("returns the indexed reward manager state", func(t *testing.T) {
		seedRewardManagerInit(t, pool, launchedMint, onChainRewardManager)
		rm, err := ResolveRewardManager(ctx, pool, launchedMint)
		require.NoError(t, err)
		assert.Equal(t, onChainRewardManager, rm)
	})

	t.Run("errors when the mint has never been indexed", func(t *testing.T) {
		_, err := ResolveRewardManager(ctx, pool, "NotAnIndexedMint")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRewardManagerNotIndexed)
	})
}
