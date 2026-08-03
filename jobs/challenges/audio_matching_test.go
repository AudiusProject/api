package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAudioMatching_BuyerAndSeller — one purchase mints two user_challenges
// rows: 'b' filed under the buyer, 's' filed under the (verified) seller.
// Amount math: catalog × dollars (b=1×, s=5×).
func TestAudioMatching_BuyerAndSeller(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_am", "number": 1}},
		"users": {
			{"user_id": 1400, "wallet": "0x1400", "is_verified": false}, // buyer
			{"user_id": 1401, "wallet": "0x1401", "is_verified": true},  // seller (verified)
		},
		"tracks": {{"track_id": 140000, "owner_id": 1401, "title": "Premium", "blocknumber": 1}},
	})

	// sol_purchases is the source-of-truth; v_usdc_purchases is a view over it.
	// amount = 10 dollars = 10_000_000 micro-USDC.
	_, err := pool.Exec(ctx, `
		INSERT INTO sol_purchases
		   (signature, instruction_index, amount, slot, from_account, content_type, content_id,
		    buyer_user_id, access_type, valid_after_blocknumber, is_valid, created_at)
		VALUES ('sig-am-1', 0, 10000000, 1000, 'from_account', 'track', 140000,
		        1400, 'stream', 0, true, now())
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewAudioMatchingBuyerProcessor())
	runProcessor(t, pool, NewAudioMatchingSellerProcessor())

	spec := fmt.Sprintf("%x:%x", 1400, 140000)

	rb, ok := queryUserChallenge(t, pool, "b", spec)
	if assert.True(t, ok) {
		assert.True(t, rb.IsComplete)
		assert.Equal(t, int64(1400), rb.UserID, "b filed under buyer")
		assert.Equal(t, int32(10), rb.Amount, "$10 × 1 = 10 AUDIO")
	}

	rs, ok := queryUserChallenge(t, pool, "s", spec)
	if assert.True(t, ok) {
		assert.True(t, rs.IsComplete)
		assert.Equal(t, int64(1401), rs.UserID, "s filed under seller")
		assert.Equal(t, int32(50), rs.Amount, "$10 × 5 = 50 AUDIO")
	}
}

// TestAudioMatching_SellerUnverifiedNoRow — unverified seller gets no 's'
// row; buyer still earns 'b'.
func TestAudioMatching_SellerUnverifiedNoRow(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_am2", "number": 1}},
		"users": {
			{"user_id": 1410, "wallet": "0x1410"},
			{"user_id": 1411, "wallet": "0x1411", "is_verified": false},
		},
		"tracks": {{"track_id": 141000, "owner_id": 1411, "title": "X", "blocknumber": 1}},
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO sol_purchases
		   (signature, instruction_index, amount, slot, from_account, content_type, content_id,
		    buyer_user_id, access_type, valid_after_blocknumber, is_valid, created_at)
		VALUES ('sig-am-2', 0, 5000000, 1100, 'from', 'track', 141000,
		        1410, 'stream', 0, true, now())
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewAudioMatchingBuyerProcessor())
	runProcessor(t, pool, NewAudioMatchingSellerProcessor())

	spec := fmt.Sprintf("%x:%x", 1410, 141000)
	_, hasB := queryUserChallenge(t, pool, "b", spec)
	_, hasS := queryUserChallenge(t, pool, "s", spec)
	assert.True(t, hasB, "buyer still earns")
	assert.False(t, hasS, "unverified seller does not earn s")
}

// TestAudioMatching_InvalidPurchaseExcluded — sol_purchases rows with
// is_valid=false are filtered out by v_usdc_purchases.
func TestAudioMatching_InvalidPurchaseExcluded(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_am3", "number": 1}},
		"users":  {{"user_id": 1420, "wallet": "0x1420"}, {"user_id": 1421, "wallet": "0x1421", "is_verified": true}},
		"tracks": {{"track_id": 142000, "owner_id": 1421, "title": "X", "blocknumber": 1}},
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO sol_purchases
		   (signature, instruction_index, amount, slot, from_account, content_type, content_id,
		    buyer_user_id, access_type, valid_after_blocknumber, is_valid, created_at)
		VALUES ('sig-am-3', 0, 10000000, 1200, 'from', 'track', 142000,
		        1420, 'stream', 0, false, now())
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewAudioMatchingBuyerProcessor())

	_, ok := queryUserChallenge(t, pool, "b", fmt.Sprintf("%x:%x", 1420, 142000))
	assert.False(t, ok, "invalid purchase should not earn")
}
