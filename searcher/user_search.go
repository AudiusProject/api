package searcher

import "github.com/aquasecurity/esquery"

type UserSearchQuery struct {
	Query string `json:"query"`
}

func (q *UserSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.Query != "" {
		builder.Must(esquery.MultiMatch(q.Query).Fields("name", "handle"))
	}

	return builder.Map()
}
