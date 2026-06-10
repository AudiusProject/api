package testdata

var Events = []map[string]any{
	{
		"event_id":    1,
		"entity_type": "track",
		"user_id":     200,
		"entity_id":   100,
		"event_type":  "remix_contest",
		"is_deleted":  false,
	},
	{
		"event_id":    2,
		"entity_type": "track",
		"user_id":     200,
		"entity_id":   100,
		"event_type":  "live_event",
		"is_deleted":  false,
	},
	{
		"event_id":    3,
		"entity_type": "track",
		"user_id":     200,
		"entity_id":   100,
		"event_type":  "remix_contest",
		"is_deleted":  true,
	},
	{
		"event_id":    4,
		"entity_type": "track",
		"user_id":     200,
		"entity_id":   101,
		"event_type":  "remix_contest",
		"is_deleted":  false,
	},
	{
		"event_id":    5,
		"entity_type": "track",
		"user_id":     200,
		"entity_id":   101,
		"event_type":  "live_event",
		"is_deleted":  false,
	},
	{
		"event_id":    6,
		"entity_type": "track",
		"user_id":     201,
		"entity_id":   102,
		"event_type":  "remix_contest",
		"is_deleted":  false,
	},
}

// EventRoutes seeds event_routes rows that match the Events fixtures above.
// slug is keyed by event_id so tests can assert permalink construction.
var EventRoutes = []map[string]any{
	{"event_id": 1, "owner_id": 200, "slug": "summer-remix-contest"},
	{"event_id": 2, "owner_id": 200, "slug": "live-at-the-venue"},
	{"event_id": 4, "owner_id": 200, "slug": "fall-remix-contest"},
	{"event_id": 5, "owner_id": 200, "slug": "live-fall-show"},
	{"event_id": 6, "owner_id": 201, "slug": "indie-remix-contest"},
}
