package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// TestUserContestsScopedToHost verifies the per-user contest endpoint filters
// to events hosted by the requested user, and preserves the same ordering and
// `related` payload shape as the global discovery endpoint so the web client
// can swap call sites without changing render logic.
func TestUserContestsScopedToHost(t *testing.T) {
	app := emptyTestApp(t)

	hostA := 7001
	hostB := 7002 // contests we should NOT see

	hostATrackActive := 6001
	hostATrackEnded := 6002
	hostBTrack := 6003

	activeStart := parseTime(t, "2024-01-02")
	activeEnd := parseTime(t, "2099-01-01")
	endedStart := parseTime(t, "2024-02-02")
	endedEnd := parseTime(t, "2024-02-10")

	fixtures := database.FixtureMap{
		"events": []map[string]any{
			{
				"event_id":    701,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   hostATrackActive,
				"user_id":     hostA,
				"created_at":  activeStart,
				"end_date":    activeEnd,
			},
			{
				"event_id":    702,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   hostATrackEnded,
				"user_id":     hostA,
				"created_at":  endedStart,
				"end_date":    endedEnd,
			},
			{
				"event_id":    703,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   hostBTrack,
				"user_id":     hostB,
				"created_at":  activeStart,
				"end_date":    activeEnd,
			},
		},
		"users": []map[string]any{
			{"user_id": hostA, "handle": "hosta", "handle_lc": "hosta"},
			{"user_id": hostB, "handle": "hostb", "handle_lc": "hostb"},
		},
		"tracks": []map[string]any{
			{
				"track_id":   hostATrackActive,
				"owner_id":   hostA,
				"title":      "A active",
				"created_at": activeStart,
			},
			{
				"track_id":   hostATrackEnded,
				"owner_id":   hostA,
				"title":      "A ended",
				"created_at": endedStart,
			},
			{
				"track_id":   hostBTrack,
				"owner_id":   hostB,
				"title":      "B active",
				"created_at": activeStart,
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	hostAHash := trashid.MustEncodeHashID(hostA)
	hostBHash := trashid.MustEncodeHashID(hostB)
	activeEventHash := trashid.MustEncodeHashID(701)
	endedEventHash := trashid.MustEncodeHashID(702)
	otherHostEventHash := trashid.MustEncodeHashID(703)

	t.Run("scoped to host: by id, active before ended", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostAHash+"/contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          2,
			"data.0.event_id": activeEventHash,
			"data.1.event_id": endedEventHash,
		})
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, otherHostEventHash,
			"per-user endpoint must not leak contests hosted by other users")
	})

	t.Run("scoped to host: by handle", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/handle/hosta/contests")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          2,
			"data.0.event_id": activeEventHash,
			"data.1.event_id": endedEventHash,
		})
	})

	t.Run("status=active filters out ended", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostAHash+"/contests?status=active")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": activeEventHash,
		})
	})

	t.Run("status=ended filters out active", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostAHash+"/contests?status=ended")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": endedEventHash,
		})
	})

	t.Run("related payload matches discovery shape", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostAHash+"/contests")
		assert.Equal(t, 200, status)

		// Both parent tracks and the host user should be present.
		trackIds := pluckStrings(body, "related.tracks.#.id")
		assert.ElementsMatch(t,
			[]string{trashid.MustEncodeHashID(hostATrackActive), trashid.MustEncodeHashID(hostATrackEnded)},
			trackIds,
		)
		userIds := pluckStrings(body, "related.users.#.id")
		assert.Contains(t, userIds, hostAHash)
		assert.NotContains(t, userIds, hostBHash)
	})

	t.Run("user with no contests returns empty data", func(t *testing.T) {
		// Add a user with no events
		database.Seed(app.pool.Replicas[0], database.FixtureMap{
			"users": []map[string]any{
				{"user_id": 7003, "handle": "lonely", "handle_lc": "lonely"},
			},
		})
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(7003)+"/contests")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#": 0,
		})
	})
}
