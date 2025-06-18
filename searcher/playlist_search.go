package searcher

import (
	"fmt"

	"github.com/aquasecurity/esquery"
)

type PlaylistSearchQuery struct {
	Query        string
	Genres       []string
	Moods        []string
	IsTagSearch  bool
	IsAlbum      bool
	OnlyVerified bool
	MyID         int32
}

func (q *PlaylistSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.IsTagSearch {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("title", "user.handle", "user.name").Type(esquery.MatchTypeBoolPrefix))
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

	// todo: mood, genre, tags

	if q.OnlyVerified {
		builder.Must(esquery.Term("tracks.user.is_verified", true))
	}

	return builder.Map()
}
