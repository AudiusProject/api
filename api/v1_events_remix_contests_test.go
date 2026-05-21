package api

import (
	"context"
	"testing"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func pluckStrings(body []byte, path string) []string {
	values := gjson.GetBytes(body, path).Array()
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.String())
	}
	return out
}

// TestRemixContestsDiscoveryPage exercises the discovery-page endpoint:
//   - ordering (active first by soonest end_date, then ended DESC)
//   - `related.users` contains contest hosts *and* track owners (when they
//     differ — the common case is that hosts own the track, but we don't
//     want the UI to round-trip for owners when they don't)
//   - `related.tracks` contains the full track objects
//   - `related.entry_counts` counts remixes created inside the contest
//     window, matching the filter used by GET /tracks/{id}/remixes with
//     only_contest_entries=true
//   - pagination via limit/offset
//   - status filter (active|ended|all)
func TestRemixContestsDiscoveryPage(t *testing.T) {
	app := emptyTestApp(t)

	hostID := 9001
	ownerID := 9002 // track owner != contest host, exercises the owner-fold path
	remixer1 := 9003
	remixer2 := 9004

	activeTrackID := 8001
	endedTrackID := 8002

	activeStart := parseTime(t, "2024-01-02")
	activeEnd := parseTime(t, "2099-01-01") // far future => active
	endedStart := parseTime(t, "2024-02-02")
	endedEnd := parseTime(t, "2024-02-10")

	inWindowCreated := parseTime(t, "2024-01-03")
	outsideWindowCreated := parseTime(t, "2024-01-01") // before contest start

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id":    501,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   activeTrackID,
				"user_id":     hostID,
				"created_at":  activeStart,
				"end_date":    activeEnd,
			},
			{
				"event_id":    502,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   endedTrackID,
				"user_id":     hostID,
				"created_at":  endedStart,
				"end_date":    endedEnd,
			},
		},
		"users": []map[string]any{
			{"user_id": hostID, "handle": "host"},
			{"user_id": ownerID, "handle": "owner"},
			{"user_id": remixer1, "handle": "remixer1"},
			{"user_id": remixer2, "handle": "remixer2"},
		},
		"tracks": []map[string]any{
			{
				"track_id":   activeTrackID,
				"owner_id":   ownerID,
				"title":      "Active Parent",
				"created_at": activeStart,
			},
			{
				"track_id":   endedTrackID,
				"owner_id":   ownerID,
				"title":      "Ended Parent",
				"created_at": endedStart,
			},
			// Two valid remix entries for the active contest
			{
				"track_id":   8101,
				"owner_id":   remixer1,
				"title":      "Active Remix In Window A",
				"created_at": inWindowCreated,
			},
			{
				"track_id":   8102,
				"owner_id":   remixer2,
				"title":      "Active Remix In Window B",
				"created_at": inWindowCreated,
			},
			// One remix submitted *before* the contest started — must not
			// be counted.
			{
				"track_id":   8103,
				"owner_id":   remixer1,
				"title":      "Active Remix Too Early",
				"created_at": outsideWindowCreated,
			},
			// One remix for the ended contest.
			{
				"track_id":   8104,
				"owner_id":   remixer1,
				"title":      "Ended Remix",
				"created_at": parseTime(t, "2024-02-05"),
			},
		},
		"remixes": []map[string]any{
			{"parent_track_id": activeTrackID, "child_track_id": 8101},
			{"parent_track_id": activeTrackID, "child_track_id": 8102},
			{"parent_track_id": activeTrackID, "child_track_id": 8103},
			{"parent_track_id": endedTrackID, "child_track_id": 8104},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	hostHash := trashid.MustEncodeHashID(hostID)
	ownerHash := trashid.MustEncodeHashID(ownerID)
	activeTrackHash := trashid.MustEncodeHashID(activeTrackID)
	endedTrackHash := trashid.MustEncodeHashID(endedTrackID)
	activeEventHash := trashid.MustEncodeHashID(501)
	endedEventHash := trashid.MustEncodeHashID(502)

	t.Run("default ordering: active before ended", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":            2,
			"data.0.event_id":   activeEventHash,
			"data.0.entity_id":  activeTrackHash,
			"data.1.event_id":   endedEventHash,
			"data.1.entity_id":  endedTrackHash,
		})
	})

	t.Run("related.tracks contains full track objects", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		// Both parent tracks must be present. Order isn't guaranteed
		// (we iterate a Go map), so assert set membership.
		jsonAssert(t, body, map[string]any{
			"related.tracks.#": 2,
		})
		trackIds := pluckStrings(body,"related.tracks.#.id")
		assert.ElementsMatch(t,
			[]string{activeTrackHash, endedTrackHash},
			trackIds,
		)
	})

	t.Run("related.users contains host AND owner", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		userIds := pluckStrings(body,"related.users.#.id")
		assert.Contains(t, userIds, hostHash)
		assert.Contains(t, userIds, ownerHash,
			"track owner must be folded into related.users even when the contest host differs")
	})

	t.Run("related.entry_counts only counts in-window remixes", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		// Active: 2 in-window entries + 1 pre-window (excluded) = 2
		// Ended:  1 entry created during the window
		jsonAssert(t, body, map[string]any{
			"related.entry_counts." + activeTrackHash: float64(2),
			"related.entry_counts." + endedTrackHash:  float64(1),
		})
	})

	t.Run("status=active filters out ended contests", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests?status=active")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": activeEventHash,
		})
	})

	t.Run("status=ended filters out active contests", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests?status=ended")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": endedEventHash,
		})
	})

	t.Run("pagination: limit=1 returns first page only", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests?limit=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": activeEventHash,
		})
	})

	t.Run("pagination: offset=1 skips first page", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": endedEventHash,
		})
	})
}

// TestRemixContestsSortPriority covers the four-group sort:
//  1. Open + Audius-followed host.
//  2. Open + unfollowed host.
//  3. Ended + Audius-followed host.
//  4. Ended + unfollowed host.
//
// Within each group, contests sort by entry_count DESC.
func TestRemixContestsSortPriority(t *testing.T) {
	app := emptyTestApp(t)

	audiusUserID := 9100 // the "Audius" account whose follows reorder each open/ended group
	followedHostID := 9101
	unfollowedHostID := 9102
	remixerID := 9104

	// Two contests per group so we can also exercise the entry_count DESC
	// tiebreak inside each group.
	openFollowedHighEntriesTrackID := 8201 // group 1, 2 entries
	openFollowedLowEntriesTrackID := 8202  // group 1, 0 entries
	openUnfollowedHighTrackID := 8203      // group 2, 2 entries
	openUnfollowedLowTrackID := 8204       // group 2, 0 entries
	endedFollowedHighTrackID := 8205       // group 3, 2 entries
	endedFollowedLowTrackID := 8206        // group 3, 0 entries
	endedUnfollowedHighTrackID := 8207     // group 4, 2 entries
	endedUnfollowedLowTrackID := 8208      // group 4, 0 entries

	farFuture := parseTime(t, "2099-01-01")
	farPast := parseTime(t, "2024-02-10")
	contestStart := parseTime(t, "2024-01-02")
	inWindow := parseTime(t, "2024-01-03")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id": 601, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": openFollowedHighEntriesTrackID, "user_id": followedHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 602, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": openFollowedLowEntriesTrackID, "user_id": followedHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 603, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": openUnfollowedHighTrackID, "user_id": unfollowedHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 604, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": openUnfollowedLowTrackID, "user_id": unfollowedHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 605, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": endedFollowedHighTrackID, "user_id": followedHostID,
				"created_at": contestStart, "end_date": farPast,
			},
			{
				"event_id": 606, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": endedFollowedLowTrackID, "user_id": followedHostID,
				"created_at": contestStart, "end_date": farPast,
			},
			{
				"event_id": 607, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": endedUnfollowedHighTrackID, "user_id": unfollowedHostID,
				"created_at": contestStart, "end_date": farPast,
			},
			{
				"event_id": 608, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": endedUnfollowedLowTrackID, "user_id": unfollowedHostID,
				"created_at": contestStart, "end_date": farPast,
			},
		},
		"users": []map[string]any{
			{"user_id": audiusUserID, "handle": "audius"},
			{"user_id": followedHostID, "handle": "followed"},
			{"user_id": unfollowedHostID, "handle": "unfollowed"},
			{"user_id": remixerID, "handle": "remixer"},
		},
		"follows": []map[string]any{
			{"follower_user_id": audiusUserID, "followee_user_id": followedHostID},
		},
		"tracks": []map[string]any{
			{"track_id": openFollowedHighEntriesTrackID, "owner_id": followedHostID, "created_at": contestStart},
			{"track_id": openFollowedLowEntriesTrackID, "owner_id": followedHostID, "created_at": contestStart},
			{"track_id": openUnfollowedHighTrackID, "owner_id": unfollowedHostID, "created_at": contestStart},
			{"track_id": openUnfollowedLowTrackID, "owner_id": unfollowedHostID, "created_at": contestStart},
			{"track_id": endedFollowedHighTrackID, "owner_id": followedHostID, "created_at": contestStart},
			{"track_id": endedFollowedLowTrackID, "owner_id": followedHostID, "created_at": contestStart},
			{"track_id": endedUnfollowedHighTrackID, "owner_id": unfollowedHostID, "created_at": contestStart},
			{"track_id": endedUnfollowedLowTrackID, "owner_id": unfollowedHostID, "created_at": contestStart},
			// Two entries each for the "high" contest in every group.
			{"track_id": 8311, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8312, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8313, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8314, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8315, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8316, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8317, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8318, "owner_id": remixerID, "created_at": inWindow},
		},
		"remixes": []map[string]any{
			{"parent_track_id": openFollowedHighEntriesTrackID, "child_track_id": 8311},
			{"parent_track_id": openFollowedHighEntriesTrackID, "child_track_id": 8312},
			{"parent_track_id": openUnfollowedHighTrackID, "child_track_id": 8313},
			{"parent_track_id": openUnfollowedHighTrackID, "child_track_id": 8314},
			{"parent_track_id": endedFollowedHighTrackID, "child_track_id": 8315},
			{"parent_track_id": endedFollowedHighTrackID, "child_track_id": 8316},
			{"parent_track_id": endedUnfollowedHighTrackID, "child_track_id": 8317},
			{"parent_track_id": endedUnfollowedHighTrackID, "child_track_id": 8318},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	openFollowedHigh := trashid.MustEncodeHashID(601)
	openFollowedLow := trashid.MustEncodeHashID(602)
	openUnfollowedHigh := trashid.MustEncodeHashID(603)
	openUnfollowedLow := trashid.MustEncodeHashID(604)
	endedFollowedHigh := trashid.MustEncodeHashID(605)
	endedFollowedLow := trashid.MustEncodeHashID(606)
	endedUnfollowedHigh := trashid.MustEncodeHashID(607)
	endedUnfollowedLow := trashid.MustEncodeHashID(608)

	t.Run("open-vs-ended × followed-vs-unfollowed, entries desc within group", func(t *testing.T) {
		prev := config.Cfg.FeaturedAudienceUserID
		config.Cfg.FeaturedAudienceUserID = int32(audiusUserID)
		t.Cleanup(func() { config.Cfg.FeaturedAudienceUserID = prev })

		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          8,
			"data.0.event_id": openFollowedHigh,    // group 1, 2 entries
			"data.1.event_id": openFollowedLow,     // group 1, 0 entries
			"data.2.event_id": openUnfollowedHigh,  // group 2, 2 entries
			"data.3.event_id": openUnfollowedLow,   // group 2, 0 entries
			"data.4.event_id": endedFollowedHigh,   // group 3, 2 entries
			"data.5.event_id": endedFollowedLow,    // group 3, 0 entries
			"data.6.event_id": endedUnfollowedHigh, // group 4, 2 entries
			"data.7.event_id": endedUnfollowedLow,  // group 4, 0 entries
		})
	})

	t.Run("with audius user unset, only the open-vs-ended split applies", func(t *testing.T) {
		prev := config.Cfg.FeaturedAudienceUserID
		config.Cfg.FeaturedAudienceUserID = 0
		t.Cleanup(func() { config.Cfg.FeaturedAudienceUserID = prev })

		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		// Without the Audius user, the follow-based tiebreak collapses. The
		// remaining order is: open (4 rows) → ended (4 rows), each block sorted
		// by entry_count DESC then event_id ASC.
		jsonAssert(t, body, map[string]any{
			"data.#": 8,
			// Open contests with 2 entries (601, 603), then with 0 (602, 604).
			"data.0.event_id": openFollowedHigh,   // 601, 2 entries
			"data.1.event_id": openUnfollowedHigh, // 603, 2 entries
			"data.2.event_id": openFollowedLow,    // 602, 0 entries
			"data.3.event_id": openUnfollowedLow,  // 604, 0 entries
			// Ended contests with 2 entries (605, 607), then with 0 (606, 608).
			"data.4.event_id": endedFollowedHigh,   // 605
			"data.5.event_id": endedUnfollowedHigh, // 607
			"data.6.event_id": endedFollowedLow,    // 606
			"data.7.event_id": endedUnfollowedLow,  // 608
		})
	})
}

// TestRemixContestsFollowFilterIgnoresStaleRows verifies that the
// follow-based tiebreak only counts live `follows` rows. A soft-deleted
// (is_delete=true) or non-current (is_current=false) follow must not promote
// a host into the "followed" sub-group.
func TestRemixContestsFollowFilterIgnoresStaleRows(t *testing.T) {
	app := emptyTestApp(t)

	audiusUserID := 9200
	liveFollowHostID := 9201
	deletedFollowHostID := 9202
	staleFollowHostID := 9203
	remixerID := 9210

	liveTrackID := 8401
	deletedTrackID := 8402
	staleTrackID := 8403

	farFuture := parseTime(t, "2099-01-01")
	contestStart := parseTime(t, "2024-01-02")
	inWindow := parseTime(t, "2024-01-03")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			// All three contests are open with the same entry_count (1), so the
			// only differentiator left is the follow tiebreak.
			{
				"event_id": 701, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": liveTrackID, "user_id": liveFollowHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 702, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": deletedTrackID, "user_id": deletedFollowHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 703, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": staleTrackID, "user_id": staleFollowHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
		},
		"users": []map[string]any{
			{"user_id": audiusUserID, "handle": "audius"},
			{"user_id": liveFollowHostID, "handle": "live_follow"},
			{"user_id": deletedFollowHostID, "handle": "deleted_follow"},
			{"user_id": staleFollowHostID, "handle": "stale_follow"},
			{"user_id": remixerID, "handle": "remixer"},
		},
		"follows": []map[string]any{
			{"follower_user_id": audiusUserID, "followee_user_id": liveFollowHostID},
			{"follower_user_id": audiusUserID, "followee_user_id": deletedFollowHostID, "is_delete": true},
			{"follower_user_id": audiusUserID, "followee_user_id": staleFollowHostID, "is_current": false},
		},
		"tracks": []map[string]any{
			{"track_id": liveTrackID, "owner_id": liveFollowHostID, "created_at": contestStart},
			{"track_id": deletedTrackID, "owner_id": deletedFollowHostID, "created_at": contestStart},
			{"track_id": staleTrackID, "owner_id": staleFollowHostID, "created_at": contestStart},
			// One entry per contest so all three reach the same entry_count.
			{"track_id": 8411, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8412, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8413, "owner_id": remixerID, "created_at": inWindow},
		},
		"remixes": []map[string]any{
			{"parent_track_id": liveTrackID, "child_track_id": 8411},
			{"parent_track_id": deletedTrackID, "child_track_id": 8412},
			{"parent_track_id": staleTrackID, "child_track_id": 8413},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	liveEvent := trashid.MustEncodeHashID(701)
	deletedEvent := trashid.MustEncodeHashID(702)
	staleEvent := trashid.MustEncodeHashID(703)

	prev := config.Cfg.FeaturedAudienceUserID
	config.Cfg.FeaturedAudienceUserID = int32(audiusUserID)
	t.Cleanup(func() { config.Cfg.FeaturedAudienceUserID = prev })

	status, body := testGet(t, app, "/v1/events/remix-contests")
	assert.Equal(t, 200, status)

	// liveFollowHost should be the only one promoted to the followed sub-group.
	// The other two are unfollowed and break by event_id ASC (702 < 703).
	jsonAssert(t, body, map[string]any{
		"data.#":          3,
		"data.0.event_id": liveEvent,    // 701 — only live follow row
		"data.1.event_id": deletedEvent, // 702 — follow soft-deleted, treated as unfollowed
		"data.2.event_id": staleEvent,   // 703 — follow not current, treated as unfollowed
	})
}

// TestRemixContestsExcludesUnavailableContent covers server-side filtering
// of contests whose track or host is not in a publishable state. The
// frontend used to drop these on the client (the "deleted accounts surface
// contests" workaround in useAllRemixContests); the backend is now the
// source of truth.
func TestRemixContestsExcludesUnavailableContent(t *testing.T) {
	app := emptyTestApp(t)

	activeHostID := 9501
	deactivatedHostID := 9502
	unavailableHostID := 9503

	visibleTrackID := 8501
	deletedTrackID := 8502
	unlistedTrackID := 8503
	deactivatedHostTrackID := 8504
	unavailableHostTrackID := 8505

	start := parseTime(t, "2024-01-02")
	end := parseTime(t, "2099-01-01")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id": 701, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": visibleTrackID, "user_id": activeHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 702, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": deletedTrackID, "user_id": activeHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 703, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": unlistedTrackID, "user_id": activeHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 704, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": deactivatedHostTrackID, "user_id": deactivatedHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 705, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": unavailableHostTrackID, "user_id": unavailableHostID,
				"created_at": start, "end_date": end,
			},
		},
		"users": []map[string]any{
			{"user_id": activeHostID, "handle": "active_host"},
			{"user_id": deactivatedHostID, "handle": "deactivated_host", "is_deactivated": true},
			{"user_id": unavailableHostID, "handle": "unavailable_host", "is_available": false},
		},
		"tracks": []map[string]any{
			{"track_id": visibleTrackID, "owner_id": activeHostID, "created_at": start},
			{"track_id": deletedTrackID, "owner_id": activeHostID, "created_at": start, "is_delete": true},
			{"track_id": unlistedTrackID, "owner_id": activeHostID, "created_at": start, "is_unlisted": true},
			{"track_id": deactivatedHostTrackID, "owner_id": deactivatedHostID, "created_at": start},
			{"track_id": unavailableHostTrackID, "owner_id": unavailableHostID, "created_at": start},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	visibleEventHash := trashid.MustEncodeHashID(701)

	t.Run("only the contest with a visible track and active host is returned", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": visibleEventHash,
		})
	})

	t.Run("deleted track contest is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(702),
			"contest pointing at a deleted track must not be returned")
	})

	t.Run("unlisted track contest is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(703),
			"contest pointing at an unlisted track must not be returned")
	})

	t.Run("deactivated host contest is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(704),
			"contest hosted by a deactivated user must not be returned")
	})

	t.Run("unavailable (deleted) host contest is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(705),
			"contest hosted by a user with is_available=false must not be returned")
	})
}

// TestRemixContestsExcludesShadowbannedHosts covers the two parallel
// shadow-ban signals applied to the discovery list:
//  1. `karma_reported_authors` — host authored a comment that crossed the
//     high-karma-reporter threshold (sum of reporters' follower_count
//     >= karmaCommentCountThreshold). Same threshold that hides the
//     comment itself on comment endpoints, lifted to author user_id.
//  2. The karma-muted set — host has been muted by users whose combined
//     follower_count crosses karmaCommentCountThreshold.
func TestRemixContestsExcludesShadowbannedHosts(t *testing.T) {
	app := emptyTestApp(t)

	cleanHostID := 9601
	karmaReportedHostID := 9602
	karmaMutedHostID := 9603
	highKarmaUserID := 9604

	cleanTrackID := 8601
	karmaReportedTrackID := 8602
	karmaMutedTrackID := 8603

	// Comment authored by karmaReportedHost on its own track; the high-karma
	// user reports it, which should cross the threshold and propagate the
	// shadow-ban from comment_id up to the host's user_id.
	reportedCommentID := 7701

	start := parseTime(t, "2024-01-02")
	end := parseTime(t, "2099-01-01")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id": 801, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": cleanTrackID, "user_id": cleanHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 802, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": karmaReportedTrackID, "user_id": karmaReportedHostID,
				"created_at": start, "end_date": end,
			},
			{
				"event_id": 803, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": karmaMutedTrackID, "user_id": karmaMutedHostID,
				"created_at": start, "end_date": end,
			},
		},
		"users": []map[string]any{
			{"user_id": cleanHostID, "handle": "clean_host"},
			{"user_id": karmaReportedHostID, "handle": "karma_reported_host"},
			{"user_id": karmaMutedHostID, "handle": "karma_muted_host"},
			{"user_id": highKarmaUserID, "handle": "high_karma_user"},
		},
		"tracks": []map[string]any{
			{"track_id": cleanTrackID, "owner_id": cleanHostID, "created_at": start},
			{"track_id": karmaReportedTrackID, "owner_id": karmaReportedHostID, "created_at": start},
			{"track_id": karmaMutedTrackID, "owner_id": karmaMutedHostID, "created_at": start},
		},
		"comments": []map[string]any{
			{
				"comment_id": reportedCommentID, "user_id": karmaReportedHostID,
				"entity_id": karmaReportedTrackID, "entity_type": "Track", "text": "reported comment",
			},
		},
		"comment_reports": []map[string]any{
			{"comment_id": reportedCommentID, "user_id": highKarmaUserID},
		},
		"muted_users": []map[string]any{
			// High-karma user mutes the karma-muted host — combined with the
			// follower_count bump below, this should cross the threshold.
			{"user_id": highKarmaUserID, "muted_user_id": karmaMutedHostID},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// `aggregate_user` rows are created by the users trigger; bump the
	// high-karma user's follower_count past the threshold so both
	// karma_reported_authors (via comment_reports) and muted_by_karma
	// (via muted_users) trip on their respective host.
	_, err := app.pool.Exec(context.Background(),
		`UPDATE aggregate_user SET follower_count = $1 WHERE user_id = $2`,
		karmaCommentCountThreshold+1, highKarmaUserID,
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanEventHash := trashid.MustEncodeHashID(801)

	t.Run("only the clean host's contest is returned", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": cleanEventHash,
		})
	})

	t.Run("host with karma-reported comment is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(802),
			"contest hosted by a user who authored a comment in high_karma_reporters must not be returned")
	})

	t.Run("karma-muted host is excluded", func(t *testing.T) {
		_, body := testGet(t, app, "/v1/events/remix-contests")
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, trashid.MustEncodeHashID(803),
			"contest hosted by a user in muted_by_karma must not be returned")
	})
}
