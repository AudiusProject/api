package searcher

import (
	"testing"
)

func TestPlaylistDsl(t *testing.T) {
	q := &PlaylistSearchQuery{
		Query: "rap",
	}

	dsl := BuildFunctionScoreDSL("repost_count", q.Map())
	testSearch(t, "playlists", dsl)
}
