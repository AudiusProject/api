package searcher

import (
	"testing"
)

func TestTrackDsl(t *testing.T) {

	ts := TrackSearchQuery{
		Query:       "KING",
		MinBPM:      80,
		MaxBPM:      220,
		Genres:      []string{"Rap"},
		MusicalKeys: []string{"A minor", "B minor"},
		MyID:        1,
	}

	dsl := BuildFunctionScoreDSL("repost_count", ts.Map())
	testSearch(t, "tracks", dsl)

}
