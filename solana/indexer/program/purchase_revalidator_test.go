package program

import (
	"context"
	"strconv"
	"testing"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
	"go.uber.org/zap"
)

func TestParseRevalidationPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		payload  string
		wantType string
		wantID   int32
		wantOK   bool
	}{
		{"track", "track:1234", "track", 1234, true},
		{"album", "album:42", "album", 42, true},
		{"empty", "", "", 0, false},
		{"no separator", "track1234", "", 0, false},
		{"non numeric id", "track:abc", "", 0, false},
		{"id with extra colon", "track:12:34", "", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotType, gotID, gotOK := parseRevalidationPayload(tc.payload)
			assert.Equal(t, tc.wantOK, gotOK)
			if tc.wantOK {
				assert.Equal(t, tc.wantType, gotType)
				assert.Equal(t, tc.wantID, gotID)
			}
		})
	}
}

// Direct call to revalidateContent: pending row with matching payments and a
// caught-up blocks table should flip to valid.
func TestRevalidatorRevalidateContent(t *testing.T) {
	ctx := t.Context()
	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")

	const (
		sellerUserId          = 1
		trackId               = 42
		priceCents            = 100
		validAfterBlocknumber = 200
		signature             = "test-sig-direct"
	)
	priceUsdc := float64(priceCents * 10000)
	payoutWallet := solana.NewWallet().PublicKey()
	networkSplit := int64(priceUsdc * config.Cfg.NetworkTakeRate / 100.0)
	payoutSplit := int64(priceUsdc) - networkSplit

	// FK constraint: tracks.blocknumber references blocks.number. Insert the
	// target block first so the tracks fixture can reference it.
	insertBlock(t, pool, validAfterBlocknumber)
	seedRevalidatorFixtures(pool, sellerUserId, trackId, priceCents, payoutWallet, validAfterBlocknumber, validAfterBlocknumber)
	insertPendingPurchase(t, pool, signature, trackId, validAfterBlocknumber)
	insertPaymentRows(t, pool, signature, map[string]int64{
		payoutWallet.String(): payoutSplit,
		config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String(): networkSplit,
	})

	rev := NewRevalidator(pool, config.Cfg, zap.NewNop())
	require.NoError(t, rev.revalidateContent(ctx, "track", trackId))

	isValid := scanIsValid(t, pool, signature)
	require.NotNil(t, isValid)
	assert.True(t, *isValid, "fully-paid purchase should be marked valid")
}

// End-to-end: install the trigger, start the revalidator, then UPDATE the
// tracks blocknumber. The trigger should fire NOTIFY, the listener should
// consume it, and is_valid should flip.
func TestRevalidatorEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	pool := database.CreateTestDatabase(t, "test_solana_indexer_program")

	const (
		sellerUserId          = 1
		trackId               = 99
		priceCents            = 100
		validAfterBlocknumber = 300
		signature             = "test-sig-e2e"
	)
	priceUsdc := float64(priceCents * 10000)
	payoutWallet := solana.NewWallet().PublicKey()
	networkSplit := int64(priceUsdc * config.Cfg.NetworkTakeRate / 100.0)
	payoutSplit := int64(priceUsdc) - networkSplit

	// Insert the target block first (FK from tracks.blocknumber). The Seed
	// helper auto-inserts block 101, so start the track there — it's below
	// validAfterBlocknumber, leaving room to bump it up later and drive the
	// trigger path rather than the startup sweep.
	insertBlock(t, pool, validAfterBlocknumber)
	seedRevalidatorFixtures(pool, sellerUserId, trackId, priceCents, payoutWallet, 101, validAfterBlocknumber)
	insertPendingPurchase(t, pool, signature, trackId, validAfterBlocknumber)
	insertPaymentRows(t, pool, signature, map[string]int64{
		payoutWallet.String(): payoutSplit,
		config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String(): networkSplit,
	})

	rev := NewRevalidator(pool, config.Cfg, zap.NewNop())
	rev.Start(ctx)

	// Wait for the startup sweep to settle. It would resolve this row itself
	// (blocks is already past validAfterBlocknumber), so re-pend after the
	// sweep finishes to make sure the assertion is driven purely by the
	// trigger.
	time.Sleep(300 * time.Millisecond)
	requireExec(t, pool,
		`UPDATE sol_purchases SET is_valid = NULL WHERE signature = $1`,
		signature,
	)

	requireExec(t, pool,
		`UPDATE tracks SET blocknumber = $1 WHERE track_id = $2`,
		validAfterBlocknumber, trackId,
	)

	pollUntil(t, 5*time.Second, 25*time.Millisecond, func() bool {
		isValid := scanIsValid(t, pool, signature)
		return isValid != nil && *isValid
	}, "is_valid should flip via trigger+listener")
}

// --- helpers ---

func seedRevalidatorFixtures(
	pool *pgxpool.Pool,
	sellerUserId, trackId, priceCents int,
	payoutWallet solana.PublicKey,
	trackBlocknumber int,
	priceBlocknumber int,
) {
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": sellerUserId},
		},
		"user_payout_wallet_history": {
			{
				"user_id":                sellerUserId,
				"spl_usdc_payout_wallet": payoutWallet.String(),
			},
		},
		"tracks": {
			{
				"track_id":    trackId,
				"owner_id":    sellerUserId,
				"blocknumber": trackBlocknumber,
			},
		},
		"track_price_history": {
			{
				"track_id":          trackId,
				"splits":            `[{"user_id": ` + strconv.Itoa(sellerUserId) + `, "percentage": 100}]`,
				"total_price_cents": priceCents,
				// Must be >= the purchase's valid_after_blocknumber so
				// getRelevantPrice's `blocknumber >= @blocknumber` predicate
				// matches this row.
				"blocknumber": priceBlocknumber,
			},
		},
	})
}

func insertBlock(t *testing.T, pool *pgxpool.Pool, number int) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		INSERT INTO blocks (blockhash, parenthash, number)
		VALUES ('test-block-' || $1::integer::text, NULL, $1::integer)
		ON CONFLICT DO NOTHING
	`, number)
	require.NoError(t, err)
}

func insertPendingPurchase(t *testing.T, pool *pgxpool.Pool, signature string, contentId, validAfterBlocknumber int) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		INSERT INTO sol_purchases (
			signature, instruction_index, amount, slot,
			from_account, content_type, content_id, buyer_user_id,
			access_type, valid_after_blocknumber, is_valid
		) VALUES (
			$1, 0, 1000000, 1,
			'from-account', 'track', $2, 2,
			'stream', $3, NULL
		)
	`, signature, contentId, validAfterBlocknumber)
	require.NoError(t, err)
}

func insertPaymentRows(t *testing.T, pool *pgxpool.Pool, signature string, routes map[string]int64) {
	t.Helper()
	routeIndex := 0
	for account, amount := range routes {
		_, err := pool.Exec(t.Context(), `
			INSERT INTO sol_payments (signature, instruction_index, route_index, to_account, amount, slot)
			VALUES ($1, 0, $2, $3, $4, 1)
		`, signature, routeIndex, account, amount)
		require.NoError(t, err)
		routeIndex++
	}
}

func scanIsValid(t *testing.T, pool *pgxpool.Pool, signature string) *bool {
	t.Helper()
	var isValid *bool
	err := pool.QueryRow(t.Context(),
		`SELECT is_valid FROM sol_purchases WHERE signature = $1 AND instruction_index = 0`,
		signature,
	).Scan(&isValid)
	if err == pgx.ErrNoRows {
		return nil
	}
	require.NoError(t, err)
	return isValid
}

func requireExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

// pollUntil polls cond until it returns true or timeout elapses. Replacement
// for testify Eventually (not in this fork of testify).
func pollUntil(t *testing.T, timeout, interval time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %s: %s", timeout, msg)
}
