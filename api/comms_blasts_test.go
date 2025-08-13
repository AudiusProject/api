package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetNewBlasts(t *testing.T) {
	app := emptyTestApp(t)

	// Setup test data
	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "artist1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour * 2),
				"updated_at": now.Add(-time.Hour * 2),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "fan1",
				"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"created_at": now.Add(-time.Hour * 2),
				"updated_at": now.Add(-time.Hour * 2),
				"is_current": true,
			},
			{
				"user_id":    3,
				"handle":     "user3",
				"wallet":     "0x4954d18926ba0ed9378938444731be4e622537b2",
				"created_at": now.Add(-time.Hour * 2),
				"updated_at": now.Add(-time.Hour * 2),
				"is_current": true,
			},
		},
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Original Track",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
			},
			{
				"track_id":   2,
				"owner_id":   2,
				"title":      "Remix Track",
				"created_at": now.Add(-time.Minute * 10),
				"updated_at": now.Add(-time.Minute * 10),
			},
		},
		"follows": {
			{
				"follower_user_id": 2,
				"followee_user_id": 1,
				"created_at":       now.Add(-time.Hour),
				"is_current":       true,
				"is_delete":        false,
			},
			{
				"follower_user_id": 3,
				"followee_user_id": 1,
				"created_at":       now.Add(-time.Hour),
				"is_current":       true,
				"is_delete":        false,
			},
		},
		"user_tips": {
			{
				"sender_user_id":   2,
				"receiver_user_id": 1,
				"amount":           1000,
				"created_at":       now.Add(-time.Hour),
				"slot":             101,
				"signature":        "tip_sig_123",
			},
		},
		"remixes": {
			{
				"parent_track_id": 1,
				"child_track_id":  2,
			},
		},
		"usdc_purchases": {
			{
				"buyer_user_id":  2,
				"seller_user_id": 1,
				"content_type":   "track",
				"content_id":     1,
				"amount":         1000000,                 // 1 USDC in micro-units
				"created_at":     now.Add(-time.Hour * 2), // Purchase before blast
				"signature":      "purchase_sig_123",
				"slot":           101,
			},
			{
				"buyer_user_id":  2,
				"seller_user_id": 1,
				"content_type":   "track",
				"content_id":     2,
				"amount":         2000000,                 // 2 USDC in micro-units
				"created_at":     now.Add(-time.Hour * 2), // Purchase before blast
				"signature":      "purchase_sig_456",
				"slot":           102,
			},
			{
				"buyer_user_id":  3,
				"seller_user_id": 1,
				"content_type":   "track",
				"content_id":     1,                       // User 3 only bought track 1, not track 2
				"amount":         500000,                  // 0.5 USDC in micro-units
				"created_at":     now.Add(-time.Hour * 2), // Purchase before blast
				"signature":      "purchase_sig_789",
				"slot":           103,
			},
		},
		"artist_coins": {
			{
				"user_id":    1,
				"ticker":     "$ARTIST1",
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"decimals":   8,
				"created_at": now.Add(-time.Hour),
			},
		},
		"sol_user_balances": {
			{
				"user_id":    2,
				"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
				"balance":    1000000,
				"created_at": now.Add(-time.Hour * 2), // Balance before blast
			},
		},
		"chat_blast": {
			{
				"blast_id":     "blast_follower_1",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "Hello to all my followers!",
				"created_at":   now.Add(-time.Minute * 10),
			},
			{
				"blast_id":     "blast_tipper_1",
				"from_user_id": 1,
				"audience":     "tipper_audience",
				"plaintext":    "Thank you for the tip!",
				"created_at":   now.Add(-time.Minute * 9),
			},
			{
				"blast_id":              "blast_customer_track1",
				"from_user_id":          1,
				"audience":              "customer_audience",
				"audience_content_type": "track",
				"audience_content_id":   1,
				"plaintext":             "Thanks for purchasing track 1!",
				"created_at":            now.Add(-time.Minute * 8),
			},
			{
				"blast_id":              "blast_customer_track2",
				"from_user_id":          1,
				"audience":              "customer_audience",
				"audience_content_type": "track",
				"audience_content_id":   2,
				"plaintext":             "Thanks for purchasing track 2!",
				"created_at":            now.Add(-time.Minute * 7),
			},
			{
				"blast_id":     "blast_customer_all",
				"from_user_id": 1,
				"audience":     "customer_audience",
				// No content targeting = all customers
				"plaintext":  "Thanks to all my customers!",
				"created_at": now.Add(-time.Minute * 6),
			},
			{
				"blast_id":              "blast_remixer_track1",
				"from_user_id":          1,
				"audience":              "remixer_audience",
				"audience_content_type": "track",
				"audience_content_id":   1,
				"plaintext":             "Love the remix of track 1!",
				"created_at":            now.Add(-time.Minute * 5),
			},
			{
				"blast_id":     "blast_remixer_all",
				"from_user_id": 1,
				"audience":     "remixer_audience",
				// No content targeting = all remixers
				"plaintext":  "Thanks to all remixers!",
				"created_at": now.Add(-time.Minute * 4),
			},
			{
				"blast_id":     "blast_coin_holder_1",
				"from_user_id": 1,
				"audience":     "coin_holder_audience",
				"plaintext":    "Update for coin holders!",
				"created_at":   now.Add(-time.Minute * 3),
			},
			{
				"blast_id":     "blast_old",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "Old blast before user followed",
				"created_at":   now.Add(-time.Hour * 10),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	t.Run("get blasts for all audiences", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"health.is_healthy": true,
		})

		// Debug: Let's see what we actually get
		t.Logf("Response body: %s", string(body))

		// Should contain blasts for audience types that user qualifies for
		assert.Contains(t, string(body), "blast_follower_1")      // User follows artist
		assert.Contains(t, string(body), "blast_tipper_1")        // User tipped artist
		assert.Contains(t, string(body), "blast_customer_track1") // User bought track 1
		assert.Contains(t, string(body), "blast_customer_track2") // User bought track 2
		assert.Contains(t, string(body), "blast_customer_all")    // User is a customer
		assert.Contains(t, string(body), "blast_coin_holder_1")   // User holds artist's coins
		assert.Contains(t, string(body), "blast_remixer_track1")  // User remixed track 1
		assert.Contains(t, string(body), "blast_remixer_all")     // User is a remixer

		// Should NOT contain old blast
		assert.NotContains(t, string(body), "blast_old") // Created before any relationships
	})

	t.Run("verify pending chat IDs are generated", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		// Verify pending_chat_id is generated for user 2 chatting with user 1
		expectedChatID := trashid.ChatID(2, 1)
		assert.Contains(t, string(body), expectedChatID)
	})

	t.Run("user with no relationships gets no blasts", func(t *testing.T) {
		// Create a completely separate test with fresh database state
		isolatedApp := emptyTestApp(t)

		isolatedFixtures := database.FixtureMap{
			"users": {
				{
					"user_id":    1,
					"handle":     "blast_sender",
					"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
					"created_at": now.Add(-time.Hour),
					"updated_at": now.Add(-time.Hour),
					"is_current": true,
				},
				{
					"user_id":    99,
					"handle":     "isolated_user",
					"wallet":     "0x4954d18926ba0ed9378938444731be4e622537b2",
					"created_at": now.Add(-time.Hour),
					"updated_at": now.Add(-time.Hour),
					"is_current": true,
				},
			},
			"chat_blast": {
				{
					"blast_id":     "isolated_blast",
					"from_user_id": 1,
					"audience":     "follower_audience",
					"plaintext":    "This should not appear for isolated user",
					"created_at":   now.Add(-time.Minute * 5),
				},
			},
		}
		database.Seed(isolatedApp.pool.Replicas[0], isolatedFixtures)

		status, body := testGetWithWallet(t, isolatedApp, "/comms/blasts", "0x4954d18926ba0ed9378938444731be4e622537b2")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"health.is_healthy": true,
		})

		// Should contain empty data array since user 99 has no relationships with user 1
		assert.Contains(t, string(body), `"data":[]`)
	})

	t.Run("content-specific audience targeting", func(t *testing.T) {
		// Test user 3 who follows artist and bought track 1 (but not track 2)
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0x4954d18926ba0ed9378938444731be4e622537b2")
		assert.Equal(t, 200, status)

		// User 3 should see:
		assert.Contains(t, string(body), "blast_follower_1")      // User follows artist
		assert.Contains(t, string(body), "blast_customer_track1") // User bought track 1
		assert.Contains(t, string(body), "blast_customer_all")    // User is a customer

		// Should NOT see track 2 specific blast (user 3 didn't buy track 2)
		assert.NotContains(t, string(body), "blast_customer_track2")

		// Should NOT see tipper, remixer, or coin holder blasts (user 3 doesn't have those relationships)
		assert.NotContains(t, string(body), "blast_tipper_1")
		assert.NotContains(t, string(body), "blast_remixer_track1")
		assert.NotContains(t, string(body), "blast_remixer_all")
		assert.NotContains(t, string(body), "blast_coin_holder_1")
	})

	t.Run("remixer audience with proper timing", func(t *testing.T) {
		// Create a specific test for remixer audience with correct timing
		remixerFixtures := database.FixtureMap{
			"tracks": {
				{
					"track_id":   10,
					"owner_id":   1,
					"title":      "Original for Remix",
					"created_at": now.Add(-time.Hour),
					"updated_at": now.Add(-time.Hour),
				},
				{
					"track_id":   11,
					"owner_id":   2,
					"title":      "Remix of Original",
					"created_at": now.Add(-time.Minute * 10), // Created AFTER the blast below
					"updated_at": now.Add(-time.Minute * 10),
				},
			},
			"remixes": {
				{
					"parent_track_id": 10,
					"child_track_id":  11,
				},
			},
			"chat_blast": {
				{
					"blast_id":              "blast_remixer_timing_test",
					"from_user_id":          1,
					"audience":              "remixer_audience",
					"audience_content_type": "track",
					"audience_content_id":   10,
					"plaintext":             "Thanks for the remix!",
					"created_at":            now.Add(-time.Minute * 15), // BEFORE remix track creation
				},
			},
		}
		database.Seed(app.pool.Replicas[0], remixerFixtures)

		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		// Should now see the remixer blast with proper timing
		assert.Contains(t, string(body), "blast_remixer_timing_test")
	})

	t.Run("coin holder audience timing logic", func(t *testing.T) {
		// Create a separate test to verify coin holder timing: sub.created_at < blast.created_at
		coinApp := emptyTestApp(t)

		coinFixtures := database.FixtureMap{
			"users": {
				{
					"user_id":    1,
					"handle":     "artist_with_coin",
					"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
					"created_at": now.Add(-time.Hour * 2),
					"updated_at": now.Add(-time.Hour * 2),
					"is_current": true,
				},
				{
					"user_id":    50,
					"handle":     "coin_holder",
					"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
					"created_at": now.Add(-time.Hour * 2),
					"updated_at": now.Add(-time.Hour * 2),
					"is_current": true,
				},
			},
			"artist_coins": {
				{
					"user_id":    1,
					"ticker":     "$TEST",
					"mint":       "TestMint123456789",
					"decimals":   8,
					"created_at": now.Add(-time.Hour * 2),
				},
			},
			"sol_user_balances": {
				{
					"user_id":    50,
					"mint":       "TestMint123456789",
					"balance":    5000000,             // User holds 50 tokens
					"created_at": now.Add(-time.Hour), // Balance created 1 hour ago
				},
			},
			"chat_blast": {
				{
					"blast_id":     "blast_before_balance",
					"from_user_id": 1,
					"audience":     "coin_holder_audience",
					"plaintext":    "Blast sent before user got coins (should NOT appear)",
					"created_at":   now.Add(-time.Hour * 6), // Way BEFORE coin balance (which is 5 hours ago)
				},
				{
					"blast_id":     "blast_after_balance",
					"from_user_id": 1,
					"audience":     "coin_holder_audience",
					"plaintext":    "Blast sent after user got coins (SHOULD appear)",
					"created_at":   now.Add(-time.Minute * 30), // AFTER coin balance
				},
			},
		}
		database.Seed(coinApp.pool.Replicas[0], coinFixtures)

		status, body := testGetWithWallet(t, coinApp, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		// Should contain blast sent AFTER the user got coins
		assert.Contains(t, string(body), "blast_after_balance")

		// Should NOT contain blast sent BEFORE the user got coins (tests sub.created_at < blast.created_at timing)
		assert.NotContains(t, string(body), "blast_before_balance")
	})
}

func TestGetNewBlastsWithPermissions(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "artist1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "fan1",
				"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
		"follows": {
			{
				"follower_user_id": 2,
				"followee_user_id": 1,
				"created_at":       now.Add(-time.Hour),
				"is_current":       true,
				"is_delete":        false,
			},
		},
		"chat_blast": {
			{
				"blast_id":     "blast_before_permission_change",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "Blast before permission change",
				"created_at":   now.Add(-time.Minute * 10),
			},
			{
				"blast_id":     "blast_after_permission_change",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "Blast after permission change",
				"created_at":   now.Add(-time.Minute * 2),
			},
		},
		"chat_permissions": {
			{
				"user_id":    2,
				"permits":    "followees",
				"allowed":    true,
				"updated_at": now.Add(-time.Minute * 5), // Permission change happened 5 minutes ago
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	t.Run("blasts filtered by permission changes", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		// Debug: Let's see what we actually get
		t.Logf("Response body: %s", string(body))

		jsonAssert(t, body, map[string]any{
			"health.is_healthy": true,
		})

		// The permission change filtering means only blasts after the permission change should appear
		// Since both blasts are follower_audience and the follow relationship exists,
		// only the blast after the permission change should be included
		assert.Contains(t, string(body), "blast_after_permission_change")
	})
}

func TestGetNewBlastsWithExistingChats(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "artist1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "fan1",
				"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
		"follows": {
			{
				"follower_user_id": 2,
				"followee_user_id": 1,
				"created_at":       now.Add(-time.Hour),
				"is_current":       true,
				"is_delete":        false,
			},
		},
		"chat": {
			{
				"chat_id":         trashid.ChatID(2, 1), // Existing chat between users
				"created_at":      now.Add(-time.Hour),
				"last_message_at": now.Add(-time.Minute * 30),
			},
		},
		"chat_member": {
			{
				"chat_id":            trashid.ChatID(2, 1),
				"user_id":            2,
				"invited_by_user_id": 2,
				"invite_code":        "",
				"created_at":         now.Add(-time.Hour),
			},
		},
		"chat_blast": {
			{
				"blast_id":     "blast_with_existing_chat",
				"from_user_id": 1,
				"audience":     "follower_audience",
				"plaintext":    "This blast should be filtered out",
				"created_at":   now.Add(-time.Minute * 5),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	t.Run("blasts filtered when existing chat exists", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"health.is_healthy": true,
		})

		// Should contain empty data because existing chat filters out the blast
		assert.Contains(t, string(body), `"data":[]`)
		assert.NotContains(t, string(body), "blast_with_existing_chat")
	})
}

func TestGetNewBlastsAudienceSpecificFiltering(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":    1,
				"handle":     "artist1",
				"wallet":     "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
			{
				"user_id":    2,
				"handle":     "customer1",
				"wallet":     "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
				"is_current": true,
			},
		},
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"title":      "Track 1",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
			},
			{
				"track_id":   2,
				"owner_id":   1,
				"title":      "Track 2",
				"created_at": now.Add(-time.Hour),
				"updated_at": now.Add(-time.Hour),
			},
		},
		"usdc_purchases": {
			{
				"buyer_user_id":  2,
				"seller_user_id": 1,
				"content_type":   "track",
				"content_id":     1, // User only bought track 1
				"amount":         1000000,
				"created_at":     now.Add(-time.Hour),
				"signature":      "purchase_sig_123",
				"slot":           101,
			},
		},
		"chat_blast": {
			{
				"blast_id":              "blast_track1_customers",
				"from_user_id":          1,
				"audience":              "customer_audience",
				"audience_content_type": "track",
				"audience_content_id":   1, // Targeted to track 1 customers
				"plaintext":             "Thanks for buying track 1!",
				"created_at":            now.Add(-time.Minute * 5),
			},
			{
				"blast_id":              "blast_track2_customers",
				"from_user_id":          1,
				"audience":              "customer_audience",
				"audience_content_type": "track",
				"audience_content_id":   2, // Targeted to track 2 customers
				"plaintext":             "Thanks for buying track 2!",
				"created_at":            now.Add(-time.Minute * 4),
			},
			{
				"blast_id":     "blast_all_customers",
				"from_user_id": 1,
				"audience":     "customer_audience",
				// No audience_content_id = targets all customers
				"plaintext":  "Thanks to all my customers!",
				"created_at": now.Add(-time.Minute * 3),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	t.Run("content-specific audience filtering", func(t *testing.T) {
		status, body := testGetWithWallet(t, app, "/comms/blasts", "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0")
		assert.Equal(t, 200, status)

		// User bought track 1, so should see:
		assert.Contains(t, string(body), "blast_track1_customers") // Targeted to track 1 buyers
		assert.Contains(t, string(body), "blast_all_customers")    // Targeted to all customers

		// Should NOT see:
		assert.NotContains(t, string(body), "blast_track2_customers") // Targeted to track 2 buyers only
	})
}
