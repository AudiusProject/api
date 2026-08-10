package api

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// Buying an album grants access to its tracks, and that access survives a
// track later being removed from the album — but only for buyers whose
// purchase predates the removal.
//
// The entitlement is decided from tracks.playlists_previously_containing_track,
// a jsonb object keyed by playlist id: {"<playlist_id>": {"time": <unix>}}.
//
// Two tracks leaving the same album at different times is the case that makes
// this per-(track, album) rather than per-album: a purchase can sit between
// the two removals and cover one track but not the other.
func TestTrackAccessAfterRemovalFromPurchasedAlbum(t *testing.T) {
	app := emptyTestApp(t)

	const (
		artistID = 900
		albumID  = 910
		early    = 920 // left the album first
		late     = 921 // left the album later

		boughtBeforeBoth = 901 // purchase precedes both removals
		boughtBetween    = 902 // purchase sits between the two removals
		boughtAfterBoth  = 903 // purchase follows both removals
	)
	const (
		walletBefore  = "0x4954d18926ba0ed9378938444731be4e622537b2"
		walletBetween = "0x7d273271690538cf855e5b3002a0dd8c154bb060"
		walletAfter   = "0x855d28d495ec1b06364bb7a521212753e2190b95"
	)
	var (
		removedEarly = time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
		removedLate  = time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	)

	usdcGate := map[string]any{
		"usdc_purchase": map[string]any{
			"price":  100.0,
			"splits": []map[string]any{{"user_id": artistID, "percentage": 100.0}},
		},
	}

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": artistID, "handle": "artist900", "handle_lc": "artist900", "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": boughtBeforeBoth, "handle": "buyer901", "handle_lc": "buyer901", "wallet": walletBefore},
			{"user_id": boughtBetween, "handle": "buyer902", "handle_lc": "buyer902", "wallet": walletBetween},
			{"user_id": boughtAfterBoth, "handle": "buyer903", "handle_lc": "buyer903", "wallet": walletAfter},
		},
		"playlists": []map[string]any{
			{"playlist_id": albumID, "playlist_name": "The Album", "playlist_owner_id": artistID, "is_album": true},
		},
		"tracks": []map[string]any{
			{
				"track_id": early, "owner_id": artistID, "title": "Left Early",
				"is_stream_gated": true, "stream_conditions": usdcGate,
				"playlists_previously_containing_track": map[string]any{
					strconv.Itoa(albumID): map[string]any{"time": removedEarly.Unix()},
				},
			},
			{
				"track_id": late, "owner_id": artistID, "title": "Left Late",
				"is_stream_gated": true, "stream_conditions": usdcGate,
				"playlists_previously_containing_track": map[string]any{
					strconv.Itoa(albumID): map[string]any{"time": removedLate.Unix()},
				},
			},
		},
		"sol_purchases": []map[string]any{
			{"signature": "sigbefore", "instruction_index": 0, "buyer_user_id": boughtBeforeBoth, "amount": 2000000,
				"content_type": "album", "content_id": albumID, "created_at": removedEarly.Add(-24 * time.Hour), "is_valid": true},
			{"signature": "sigbetween", "instruction_index": 0, "buyer_user_id": boughtBetween, "amount": 2000000,
				"content_type": "album", "content_id": albumID, "created_at": removedEarly.Add(24 * time.Hour), "is_valid": true},
			{"signature": "sigafter", "instruction_index": 0, "buyer_user_id": boughtAfterBoth, "amount": 2000000,
				"content_type": "album", "content_id": albumID, "created_at": removedLate.Add(24 * time.Hour), "is_valid": true},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// The viewer has to be both named (user_id, which becomes myId) and proven
	// (a signed wallet) — the query parameter alone is a 403.
	streamAccess := func(t *testing.T, trackID, viewerID int, wallet string) bool {
		t.Helper()
		status, body := testGetWithWallet(t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(trackID)+"/access-info?user_id="+trashid.MustEncodeHashID(viewerID),
			wallet)
		assert.Equal(t, 200, status)
		var resp struct {
			Data struct {
				Access struct {
					Stream bool `json:"stream"`
				} `json:"access"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode access-info: %v", err)
		}
		return resp.Data.Access.Stream
	}

	for _, tc := range []struct {
		name                string
		viewer              int
		wallet              string
		wantEarly, wantLate bool
	}{
		// Bought before either track left: both are covered.
		{"purchase precedes both removals", boughtBeforeBoth, walletBefore, true, true},
		// The discriminating case: the purchase sits between the removals, so
		// it covers only the track that was still in the album at the time.
		{"purchase between the two removals", boughtBetween, walletBetween, false, true},
		// Bought after both had left: the album never contained them for this
		// buyer.
		{"purchase follows both removals", boughtAfterBoth, walletAfter, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := streamAccess(t, early, tc.viewer, tc.wallet); got != tc.wantEarly {
				t.Errorf("track that left early: stream access = %v, want %v", got, tc.wantEarly)
			}
			if got := streamAccess(t, late, tc.viewer, tc.wallet); got != tc.wantLate {
				t.Errorf("track that left late: stream access = %v, want %v", got, tc.wantLate)
			}
		})
	}
}
