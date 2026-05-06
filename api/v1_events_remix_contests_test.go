package api

import (
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

// TestRemixContestsSortPriority covers the multi-tier sort:
//  1. Featured-audience-account contests come first.
//  2. Then contests with at least one entry.
//  3. Ended contests with zero entries land at the bottom.
//
// Within each group the existing active-first / soonest-ending sort still
// applies — we don't reassert that here because TestRemixContestsDiscoveryPage
// already covers it.
func TestRemixContestsSortPriority(t *testing.T) {
	app := emptyTestApp(t)

	featuredHostID := 9101 // contests by this user must sort first
	regularHostID := 9102
	ownerID := 9103
	remixerID := 9104

	// Track ids for each contest's parent track.
	featuredEndedZeroTrackID := 8201 // featured + ended + zero entries → still group 1
	hasEntriesActiveTrackID := 8202  // group 2 (has entries)
	hasEntriesEndedTrackID := 8203   // group 2 (has entries, ended)
	activeZeroTrackID := 8204        // group 3 (active, no entries — neither featured nor has-entries nor ended-empty)
	endedZeroTrackID := 8205         // group 4 (ended + zero entries) — must be LAST

	farFuture := parseTime(t, "2099-01-01")
	farPast := parseTime(t, "2024-02-10")
	contestStart := parseTime(t, "2024-01-02")
	inWindow := parseTime(t, "2024-01-03")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id": 601, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": featuredEndedZeroTrackID, "user_id": featuredHostID,
				"created_at": contestStart, "end_date": farPast,
			},
			{
				"event_id": 602, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": hasEntriesActiveTrackID, "user_id": regularHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 603, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": hasEntriesEndedTrackID, "user_id": regularHostID,
				"created_at": contestStart, "end_date": farPast,
			},
			{
				"event_id": 604, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": activeZeroTrackID, "user_id": regularHostID,
				"created_at": contestStart, "end_date": farFuture,
			},
			{
				"event_id": 605, "event_type": "remix_contest", "entity_type": "track",
				"entity_id": endedZeroTrackID, "user_id": regularHostID,
				"created_at": contestStart, "end_date": farPast,
			},
		},
		"users": []map[string]any{
			{"user_id": featuredHostID, "handle": "featured"},
			{"user_id": regularHostID, "handle": "regular"},
			{"user_id": ownerID, "handle": "owner"},
			{"user_id": remixerID, "handle": "remixer"},
		},
		"tracks": []map[string]any{
			{"track_id": featuredEndedZeroTrackID, "owner_id": featuredHostID, "created_at": contestStart},
			{"track_id": hasEntriesActiveTrackID, "owner_id": regularHostID, "created_at": contestStart},
			{"track_id": hasEntriesEndedTrackID, "owner_id": regularHostID, "created_at": contestStart},
			{"track_id": activeZeroTrackID, "owner_id": regularHostID, "created_at": contestStart},
			{"track_id": endedZeroTrackID, "owner_id": regularHostID, "created_at": contestStart},
			// Entries — only for the two has-entries contests.
			{"track_id": 8302, "owner_id": remixerID, "created_at": inWindow},
			{"track_id": 8303, "owner_id": remixerID, "created_at": inWindow},
		},
		"remixes": []map[string]any{
			{"parent_track_id": hasEntriesActiveTrackID, "child_track_id": 8302},
			{"parent_track_id": hasEntriesEndedTrackID, "child_track_id": 8303},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	featuredEvent := trashid.MustEncodeHashID(601)
	hasActiveEvent := trashid.MustEncodeHashID(602)
	hasEndedEvent := trashid.MustEncodeHashID(603)
	activeZeroEvent := trashid.MustEncodeHashID(604)
	endedZeroEvent := trashid.MustEncodeHashID(605)

	t.Run("featured account contests sort first, ended-zero-entries last", func(t *testing.T) {
		prev := config.Cfg.FeaturedAudienceUserID
		config.Cfg.FeaturedAudienceUserID = int32(featuredHostID)
		t.Cleanup(func() { config.Cfg.FeaturedAudienceUserID = prev })

		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          5,
			"data.0.event_id": featuredEvent,  // featured (group 1)
			"data.1.event_id": hasActiveEvent, // has entries, active (group 2)
			"data.2.event_id": hasEndedEvent,  // has entries, ended (group 2)
			"data.3.event_id": activeZeroEvent, // active, zero entries (group 3 — not ended-empty)
			"data.4.event_id": endedZeroEvent,  // ended + zero entries (group 4, LAST)
		})
	})

	t.Run("with featured user unset, featured contest falls back to entry-based sort", func(t *testing.T) {
		prev := config.Cfg.FeaturedAudienceUserID
		config.Cfg.FeaturedAudienceUserID = 0
		t.Cleanup(func() { config.Cfg.FeaturedAudienceUserID = prev })

		status, body := testGet(t, app, "/v1/events/remix-contests")
		assert.Equal(t, 200, status)

		// featuredEvent is now ended-with-zero-entries, so it should sort
		// alongside endedZeroEvent at the bottom (group 4). The two has-entries
		// contests are group 2, activeZeroEvent is group 3.
		jsonAssert(t, body, map[string]any{
			"data.#":          5,
			"data.0.event_id": hasActiveEvent,
			"data.1.event_id": hasEndedEvent,
			"data.2.event_id": activeZeroEvent,
		})
		// Last two entries are both ended-zero-entries — order within is
		// determined by end_date DESC then event_id; both events share end_date
		// (farPast), so the smaller event_id (601) comes first.
		jsonAssert(t, body, map[string]any{
			"data.3.event_id": featuredEvent,
			"data.4.event_id": endedZeroEvent,
		})
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
