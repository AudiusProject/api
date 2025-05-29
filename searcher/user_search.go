package searcher

import (
	"fmt"

	"github.com/aquasecurity/esquery"
)

type UserSearchQuery struct {
	Query string `json:"query"`
	MyID  int    `json:"my_id"`
}

func (q *UserSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.Query != "" {
		builder.Must(esquery.MultiMatch(q.Query).Fields("name", "handle").Type(esquery.MatchTypeBoolPrefix))
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

	return builder.Map()
}
