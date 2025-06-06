package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// parseTime parses a time string using RFC3339 format and fails the test if parsing fails
func parseTime(t *testing.T, timeStr string) time.Time {
	return parseTimeWithLayout(t, timeStr, time.DateOnly)
}

// parseTimeWithLayout parses a time string with the given layout and fails the test if parsing fails
func parseTimeWithLayout(t *testing.T, timeStr string, layout string) time.Time {
	t.Helper()
	parsed, err := time.Parse(layout, timeStr)
	if err != nil {
		t.Fatalf("Failed to parse time string %q: %v", timeStr, err)
	}
	return parsed
}

func TestTrackRemixes(t *testing.T) {

	app := emptyTestApp(t)
	ownerId := 5001
	firstRemixOwnerId := 5002
	secondRemixOwnerId := 5003
	thirdRemixOwnerId := 5004
	fourthRemixOwnerId := 5005
	parentTrackId := 7001
	firstRemixTrackId := 7002
	secondRemixTrackId := 7003
	thirdRemixTrackId := 7004
	fourthRemixTrackId := 7005
	fixtures := FixtureMap{
		"events": []map[string]any{
			{
				"event_id":   3,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2024-01-02"),
				"end_date":   parseTime(t, "2024-01-06"),
			},
			// Past event (should be ignored)
			{
				"event_id":   2,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2023-01-02"),
				"end_date":   parseTime(t, "2023-01-06"),
			},
			// deleted event (should be ignored)
			{
				"event_id":   1,
				"is_deleted": true,
				"event_type": "remix_contest",
				"entity_id":  parentTrackId,
				"user_id":    ownerId,
				"created_at": parseTime(t, "2023-06-01"),
				"end_date":   parseTime(t, "2023-06-06"),
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":   parentTrackId,
				"owner_id":   ownerId,
				"title":      "Parent Track",
				"created_at": parseTime(t, "2024-01-02"),
			},
			{
				"track_id":   firstRemixTrackId,
				"owner_id":   firstRemixOwnerId,
				"title":      "First Remix Track (Reposted)",
				"created_at": parseTime(t, "2024-01-03"),
			},
			{
				"track_id":   secondRemixTrackId,
				"owner_id":   secondRemixOwnerId,
				"title":      "Second Remix Track (Saved)",
				"created_at": parseTime(t, "2024-01-04"),
			},
			{
				"track_id":   thirdRemixTrackId,
				"owner_id":   thirdRemixOwnerId,
				"title":      "Third Remix Track (Too Late)",
				"created_at": parseTime(t, "2024-01-07"),
			},
			{
				"track_id":   fourthRemixTrackId,
				"owner_id":   fourthRemixOwnerId,
				"title":      "Fourth Remix Track (Too Early)",
				"created_at": parseTime(t, "2024-01-01"),
			},
		},
		"users": []map[string]any{
			{
				"user_id": ownerId,
				"handle":  "owner",
			},
			{
				"user_id": firstRemixOwnerId,
				"handle":  "first_remix_owner",
			},
			{
				"user_id": secondRemixOwnerId,
				"handle":  "second_remix_owner",
			},
			{
				"user_id": thirdRemixOwnerId,
				"handle":  "third_remix_owner",
			},
			{
				"user_id": fourthRemixOwnerId,
				"handle":  "fourth_remix_owner",
			},
		},
		"remixes": []map[string]any{
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  firstRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  secondRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  thirdRemixTrackId,
			},
			{
				"parent_track_id": parentTrackId,
				"child_track_id":  fourthRemixTrackId,
			},
		},
		"aggregate_plays": []map[string]any{
			{
				"play_item_id": firstRemixTrackId,
				"count":        100,
			},
			{
				"play_item_id": secondRemixTrackId,
				"count":        200,
			},
			{
				"play_item_id": thirdRemixTrackId,
				"count":        300,
			},
			{
				"play_item_id": fourthRemixTrackId,
				"count":        400,
			},
		},
		"reposts": []map[string]any{
			{
				"repost_item_id": firstRemixTrackId,
				"user_id":        ownerId,
				"repost_type":    "track",
				"created_at":     parseTime(t, "2024-01-03"),
			},
		},
		"saves": []map[string]any{
			{
				"save_item_id": secondRemixTrackId,
				"save_type":    "track",
				"user_id":      ownerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
			{
				"save_item_id": secondRemixTrackId,
				"save_type":    "track",
				"user_id":      firstRemixOwnerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
			{
				"save_item_id": firstRemixTrackId,
				"save_type":    "track",
				"user_id":      thirdRemixOwnerId,
				"created_at":   parseTime(t, "2024-01-04"),
			},
		},
	}

	createFixtures(app, fixtures)

	var baseUrl = "/v1/full/tracks/" + trashid.MustEncodeHashID(parentTrackId) + "/remixes"

	// Default sort: recent, no filtering
	status, body := testGet(t, app, baseUrl)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(thirdRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(firstRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(fourthRemixTrackId),
	})

	// Sort by likes
	status, body = testGet(t, app, baseUrl+"?sort_method=likes")

	// should be saves desc, then track_id desc for ties
	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(fourthRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(thirdRemixTrackId),
	})

	// Sort by plays
	status, body = testGet(t, app, baseUrl+"?sort_method=plays")

	// should be plays desc, then track_id desc for ties
	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    4,
		"data.tracks.0.id": trashid.MustEncodeHashID(fourthRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(thirdRemixTrackId),
		"data.tracks.2.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.3.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})

	// contest entries only
	status, body = testGet(t, app, baseUrl+"?sort_method=recent&only_contest_entries=true")

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    2,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})

	// cosigns only
	status, body = testGet(t, app, baseUrl+"?sort_method=recent&only_cosigns=true")

	jsonAssert(t, body, map[string]any{
		"data.tracks.#":    2,
		"data.tracks.0.id": trashid.MustEncodeHashID(secondRemixTrackId),
		"data.tracks.1.id": trashid.MustEncodeHashID(firstRemixTrackId),
	})
}

func TestTrackRemixesInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	status, _ := testGet(t, app, "/v1/full/tracks/123/remixes?sort_method=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?limit=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?limit=101")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?offset=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?only_contest_entries=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/full/tracks/123/remixes?only_cosigns=invalid")
	assert.Equal(t, 400, status)
}
