package searchv1

import (
	"fmt"

	"github.com/aquasecurity/esquery"
)

type UserSearchQuery struct {
	Query       string `json:"query"`
	IsVerified  bool   `json:"is_verified"`
	IsTagSearch bool
	Genres      []string
	MyID        int32 `json:"my_id"`
}

func (q *UserSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.IsTagSearch {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tracks.tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(esquery.MultiMatch(q.Query).
			Fields("suggest").
			MinimumShouldMatch("80%").
			Fuzziness("AUTO").
			Type(esquery.MatchTypeBoolPrefix))

		// for exact title match
		builder.Should(
			esquery.MultiMatch().Query(q.Query).
				Fields("name", "handle").
				Operator(esquery.OperatorAnd).
				Type(esquery.MatchTypePhrasePrefix),
		)
	} else {
		builder.Must(esquery.MatchAll())
	}

	if len(q.Genres) > 0 {
		builder.Must(esquery.Range("track_count").Gt(0))
		builder.Filter(esquery.Terms("tracks.genre.keyword", toAnySlice(q.Genres)...))
		// by using a match query... the TF/IDF will apply to tracks.  Which will rank profiles higher if they have a larger proportion of genre
		for _, value := range q.Genres {
			builder.Should(esquery.Match("tracks.genre", value)).Boost(10)
		}
	}

	if q.MyID > 0 {
		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "following_user_ids",
				},
				"boost": 10,
			},
		}))
	}

	if q.IsVerified {
		builder.Must(esquery.Term("is_verified", true))
	} else {
		builder.Should(esquery.Term("is_verified", true))
	}

	return builder.Map()
}

func (q *UserSearchQuery) DSL() string {
	inner := q.Map()
	return BuildFunctionScoreDSL("follower_count", inner)
}
