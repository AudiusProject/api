package searcher

import (
	"fmt"

	"github.com/aquasecurity/esquery"
)

type PlaylistSearchQuery struct {
	Query   string
	Genres  []string
	Moods   []string
	IsAlbum bool
	MyID    int32
}

func (q *PlaylistSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.Query != "" {
		builder.Must(esquery.Match("title", q.Query))
	} else {
		builder.Must(esquery.MatchAll())
	}

	builder.Filter(esquery.Term("is_album", q.IsAlbum))

	if q.MyID > 0 {
		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "reposted_playlist_ids",
				},
				"boost": 10,
			},
		}))
	}

	return builder.Map()
}
