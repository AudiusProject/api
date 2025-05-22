package searcher

import (
	"testing"

	"github.com/aquasecurity/esquery"
)

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

func TestUserDsl(t *testing.T) {
	q := &UserSearchQuery{
		Query: "ray",
	}

	dsl := functionScore("followers_count", q.Map())

	testSearch(t, "users", dsl)
}
