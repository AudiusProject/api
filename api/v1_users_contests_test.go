package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// TestGetUserContests exercises the per-user contest endpoint:
//   - filters to contests hosted by the path user (rejects another host's
//     contests)
//   - resolves both /users/handle/{handle}/contests and /users/{id}/contests
//   - keeps the same `data` + `related` shape as the global discovery
//     endpoint (so the web tab can use the same primer code path)
//   - status filter (active|ended|all) and pagination
func TestGetUserContests(t *testing.T) {
	app := emptyTestApp(t)

	hostID := 9001
	otherHostID := 9099 // contests by this user must NOT appear in /users/9001/contests
	ownerID := 9002
	remixer := 9003

	activeTrackID := 8001
	endedTrackID := 8002
	otherTrackID := 8003

	activeStart := parseTime(t, "2024-01-02")
	activeEnd := parseTime(t, "2099-01-01")
	endedStart := parseTime(t, "2024-02-02")
	endedEnd := parseTime(t, "2024-02-10")

	inWindowCreated := parseTime(t, "2024-01-03")

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
			{
				"event_id":    503,
				"event_type":  "remix_contest",
				"entity_type": "track",
				"entity_id":   otherTrackID,
				"user_id":     otherHostID,
				"created_at":  activeStart,
				"end_date":    activeEnd,
			},
		},
		"users": []map[string]any{
			{"user_id": hostID, "handle": "host", "handle_lc": "host"},
			{"user_id": otherHostID, "handle": "otherhost", "handle_lc": "otherhost"},
			{"user_id": ownerID, "handle": "owner"},
			{"user_id": remixer, "handle": "remixer"},
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
			{
				"track_id":   otherTrackID,
				"owner_id":   ownerID,
				"title":      "Other Host Parent",
				"created_at": activeStart,
			},
			{
				"track_id":   8101,
				"owner_id":   remixer,
				"title":      "Active Remix In Window",
				"created_at": inWindowCreated,
			},
		},
		"remixes": []map[string]any{
			{"parent_track_id": activeTrackID, "child_track_id": 8101},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	hostHash := trashid.MustEncodeHashID(hostID)
	activeTrackHash := trashid.MustEncodeHashID(activeTrackID)
	endedTrackHash := trashid.MustEncodeHashID(endedTrackID)
	activeEventHash := trashid.MustEncodeHashID(501)
	endedEventHash := trashid.MustEncodeHashID(502)
	otherEventHash := trashid.MustEncodeHashID(503)

	t.Run("by id: ordered active before ended, filtered to host", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":           2,
			"data.0.event_id":  activeEventHash,
			"data.0.entity_id": activeTrackHash,
			"data.1.event_id":  endedEventHash,
			"data.1.entity_id": endedTrackHash,
		})
		// other host's contest must not leak in
		eventIds := pluckStrings(body, "data.#.event_id")
		assert.NotContains(t, eventIds, otherEventHash)
	})

	t.Run("by handle: same result", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/handle/host/contests")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          2,
			"data.0.event_id": activeEventHash,
			"data.1.event_id": endedEventHash,
		})
	})

	t.Run("status=active filters out ended", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests?status=active")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": activeEventHash,
		})
	})

	t.Run("status=ended filters out active", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests?status=ended")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": endedEventHash,
		})
	})

	t.Run("pagination: limit=1 returns first page only", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests?limit=1")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": activeEventHash,
		})
	})

	t.Run("pagination: offset=1 skips first page", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests?limit=1&offset=1")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.0.event_id": endedEventHash,
		})
	})

	t.Run("related contains host + entry counts", func(t *testing.T) {
		status, body := testGet(t, app, "/v1/users/"+hostHash+"/contests")
		assert.Equal(t, 200, status)

		userIds := pluckStrings(body, "related.users.#.id")
		assert.Contains(t, userIds, hostHash)

		jsonAssert(t, body, map[string]any{
			"related.entry_counts." + activeTrackHash: float64(1),
			"related.entry_counts." + endedTrackHash:  float64(0),
		})
	})

	t.Run("user with no contests returns empty data", func(t *testing.T) {
		ownerHash := trashid.MustEncodeHashID(ownerID)
		status, body := testGet(t, app, "/v1/users/"+ownerHash+"/contests")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#": 0,
		})
	})
}
