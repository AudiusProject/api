package searcher

import (
	"testing"
)

func TestPlaylistDsl(t *testing.T) {
	t.Skip()
	q := &PlaylistSearchQuery{
		Query: "hot",
		MyID:  1,
	}

	dsl := BuildFunctionScoreDSL("repost_count", q.Map())
	testSearch(t, "playlists", dsl)
}
