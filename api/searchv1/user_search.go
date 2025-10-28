package searchv1

import (
	"fmt"
	"strings"

	"github.com/aquasecurity/esquery"
)

type UserSearchQuery struct {
	Query       string `json:"query"`
	IsVerified  bool   `json:"is_verified"`
	IsTagSearch bool
	Genres      []string
	MyID        int32 `json:"my_id"`
	SortMethod  string
}

func (q *UserSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.IsTagSearch {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tracks.tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(esquery.MultiMatch(q.Query).
			Fields("suggest", "name", "handle").
			MinimumShouldMatch("80%").
			Fuzziness("AUTO").
			Type(esquery.MatchTypeBoolPrefix))

		// for exact title match
		builder.Should(
			esquery.MultiMatch().Query(q.Query).
				Fields("name", "handle").
				Boost(1000).
				Operator(esquery.OperatorAnd),
		)

		// exact match, but remove spaces from query
		// so 'Stereo Steve' ranks 'StereoSteve' higher
		builder.Should(
			esquery.MultiMatch().Query(strings.ReplaceAll(q.Query, " ", "")).
				Fields("name", "handle").
				Boost(10).
				Operator(esquery.OperatorAnd),
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
				"boost": 1000,
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
	switch q.SortMethod {
	case "recent":
		return sortWithField(q.Map(), "created_at", "desc")
	case "popular":
		return BuildFunctionScoreDSL("follower_count", 200, inner)
	default:
		return BuildFunctionScoreDSL("follower_count", 20, inner)
	}
}
