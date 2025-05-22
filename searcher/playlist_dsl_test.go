package searcher

import (
	"testing"

	"github.com/aquasecurity/esquery"
)

type PlaylistSearchQuery struct {
	Query  string
	Genres []string
	Moods  []string
}

func (q *PlaylistSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.Query != "" {
		builder.Must(esquery.Match("title", q.Query))
	}

	return builder.Map()
}

func TestPlaylistDsl(t *testing.T) {
	q := &PlaylistSearchQuery{
		Query: "rap",
	}

	dsl := functionScore("repost_count", q.Map())
	testSearch(t, "playlists", dsl)
}
