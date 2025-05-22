package searcher

import (
	"testing"
)

func TestTrackDsl(t *testing.T) {

	ts := TrackSearchQuery{
		// Query:  "fever",
		MinBPM:      80,
		MaxBPM:      220,
		Genres:      []string{"Rap"},
		MusicalKeys: []string{"A minor", "B minor"},
	}

	dsl := BuildFunctionScoreDSL("repost_count", ts.Map())
	testSearch(t, "playlists", dsl)

}
