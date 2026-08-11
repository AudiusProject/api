// Package launchpad holds logic shared by the code paths that bind launchpad
// reward codes to a mint's Solana reward manager (the HTTP handler in
// api/v1_create_reward_code.go and the bulk CLI in cmd/create_reward_codes).
package launchpad

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"api.audius.co/utils"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
)

var (
	// ErrRewardManagerNotIndexed means no InitRewardManager instruction has
	// been indexed for the mint: either the coin was never launched, or the
	// Solana indexer hasn't caught up yet. Either way there is no reward
	// manager to bind rewards to, and guessing one by derivation is exactly
	// the mistake this package exists to prevent.
	ErrRewardManagerNotIndexed = errors.New("no indexed reward manager for mint")

	// ErrRewardManagerMismatch means the reward manager derived from the
	// currently configured launchpad secret is not the reward manager that
	// exists on Solana for this mint. That happens when the launchpad
	// deterministic secret is rotated after the coin was launched: the
	// Solana reward manager account was created once, at launch, with the
	// secret that was live then, and it can never move. Creating a pool for
	// the newly derived key would produce a pool whose reward manager has no
	// Solana account, so rewards written to it can never be redeemed.
	ErrRewardManagerMismatch = errors.New("derived reward manager does not match the mint's on-chain reward manager")
)

// Querier is the subset of pgx pool behavior needed to resolve a mint's
// reward manager. Both *pgxpool.Pool and *dbv1.DBPools satisfy it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// RewardPoolClient is the subset of *sdk.Rewards used to look up and create
// reward pools. It is an interface so the pool-creation guard below can be
// exercised without a live validator.
type RewardPoolClient interface {
	GetRewardPool(ctx context.Context, rewardsManagerPubkey string) (*v1.GetRewardPoolResponse, error)
	CreateRewardPool(ctx context.Context, msg *v1.CreateRewardPool, rmKey ed25519.PrivateKey, deadlineBlockHeight int64) (string, error)
}

// ResolveRewardManager returns the reward manager state pubkey that actually
// exists on Solana for mint, read from sol_reward_manager_inits — rows the
// Solana indexer writes from observed InitRewardManager instructions. This is
// ground truth and, unlike derivation from the launchpad deterministic
// secret, is immune to that secret being rotated. It is the same source
// redemption reads (see api/v1_coins_post_redeem.go), which is why redemption
// is unaffected by a rotation while creation is not.
//
// A mint has exactly one reward manager in practice; the ordering only makes
// the result deterministic if the indexer ever records more than one, in
// which case the earliest init is the one with the pool history.
func ResolveRewardManager(ctx context.Context, db Querier, mint string) (string, error) {
	var rewardManagerState string
	err := db.QueryRow(ctx, `
		SELECT reward_manager_state
		FROM sol_reward_manager_inits
		WHERE mint = @mint
		ORDER BY slot, signature, instruction_index
		LIMIT 1
	`, pgx.NamedArgs{"mint": mint}).Scan(&rewardManagerState)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w (mint %s): the coin was never launched, or the Solana indexer has not caught up", ErrRewardManagerNotIndexed, mint)
	}
	if err != nil {
		return "", fmt.Errorf("failed to look up reward manager for mint %s: %w", mint, err)
	}
	return rewardManagerState, nil
}

// PrepareRewardPool resolves the mint's real reward manager, ensures a
// cometbft reward pool exists for it, and returns the rewards_manager_pubkey
// the caller should use for CreateReward.
//
// Creating a pool requires an rm_owner_signature, which requires the reward
// manager PRIVATE key — and only derivation can produce that. So on the
// creation path the keypair is derived and its public half is checked against
// the reward manager that Solana actually has. A mismatch means the launchpad
// secret has been rotated since this mint launched, which is precisely the
// case where creating a pool is wrong, so it fails loudly rather than falling
// back to the derived key.
//
// For an already-launched mint the mismatch is moot: looking up the real
// reward manager means GetRewardPool finds the existing pool and the creation
// branch is never entered. For a mint launched under the current secret the
// derived key equals the real one and creation proceeds as before.
func PrepareRewardPool(
	ctx context.Context,
	logger *zap.Logger,
	db Querier,
	client RewardPoolClient,
	launchpadDeterministicSecret string,
	mint solana.PublicKey,
	claimAuthority string,
	deadlineBlockHeight int64,
) (string, error) {
	rewardsManagerPubkey, err := ResolveRewardManager(ctx, db, mint.String())
	if err != nil {
		return "", err
	}

	if _, err := client.GetRewardPool(ctx, rewardsManagerPubkey); err == nil {
		// Pool already exists — the common case for any mint that has ever
		// had a reward.
		return rewardsManagerPubkey, nil
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		return "", fmt.Errorf("failed to look up reward pool for reward manager %s: %w", rewardsManagerPubkey, err)
	}

	// Pool creation path: we need the reward manager private key, so derive
	// the keypair and verify it corresponds to the on-chain reward manager.
	rmKey := utils.DeriveRewardManagerKeypair(launchpadDeterministicSecret, mint)
	derivedPubkey := base58.Encode(rmKey.Public().(ed25519.PublicKey))
	if derivedPubkey != rewardsManagerPubkey {
		return "", fmt.Errorf(
			"%w: mint %s has on-chain reward manager %s but the configured launchpad secret derives %s; refusing to create a reward pool for a reward manager with no Solana account",
			ErrRewardManagerMismatch, mint, rewardsManagerPubkey, derivedPubkey,
		)
	}

	logger.Info("creating reward pool",
		zap.String("mint", mint.String()),
		zap.String("rewards_manager_pubkey", rewardsManagerPubkey),
		zap.String("claim_authority", claimAuthority))

	if _, createErr := client.CreateRewardPool(ctx, &v1.CreateRewardPool{
		RewardsManagerPubkey: rewardsManagerPubkey,
		Authorities:          []string{claimAuthority},
	}, rmKey, deadlineBlockHeight); createErr != nil {
		// Race window: two concurrent first-reward requests for the same
		// brand-new mint can both observe NotFound and both submit
		// CreateRewardPool. The second one fails because the pool now
		// exists. Re-fetch and treat "pool exists" as success — equivalent
		// to having lost the race cleanly. Anything else is a real error.
		if _, getErr := client.GetRewardPool(ctx, rewardsManagerPubkey); getErr != nil {
			return "", fmt.Errorf("failed to create reward pool: %w", createErr)
		}
		logger.Info("lost CreateRewardPool race; pool now exists",
			zap.String("rewards_manager_pubkey", rewardsManagerPubkey))
	}

	return rewardsManagerPubkey, nil
}
