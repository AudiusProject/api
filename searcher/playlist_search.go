package searcher

import (
	"github.com/aquasecurity/esquery"
)

type PlaylistSearchQuery struct {
	Query   string
	Genres  []string
	Moods   []string
	IsAlbum bool
}

func (q *PlaylistSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.Query != "" {
		builder.Must(esquery.Match("title", q.Query))
	}

	builder.Filter(esquery.Term("is_album", q.IsAlbum))

	return builder.Map()
}
