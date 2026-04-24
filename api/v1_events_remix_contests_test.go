package api

import (
	"testing"

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
