package searchv1

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
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tracks.tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("title", "user.handle", "user.name", "tracks.tags").Type(esquery.MatchTypeBoolPrefix))
	} else {
		builder.Must(esquery.MatchAll())
	}

	builder.Filter(esquery.Term("is_album", q.IsAlbum))

	if len(q.Genres) > 0 {
		builder.Filter(esquery.Terms("tracks.genre", toAnySlice(q.Genres)...))
	}

	if len(q.Moods) > 0 {
		builder.Filter(esquery.Terms("tracks.mood", toAnySlice(q.Moods)...))
	}

	if q.OnlyVerified {
		builder.Must(esquery.Term("tracks.user.is_verified", true))
	} else {
		builder.Should(esquery.Term("user.is_verified", true))
	}

	// personalize results
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

func (q *PlaylistSearchQuery) DSL() string {
	return BuildFunctionScoreDSL("repost_count", q.Map())
}
