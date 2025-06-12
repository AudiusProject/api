package searcher

import (
	"testing"
)

func TestUserDsl(t *testing.T) {
	t.Skip()
	q := &UserSearchQuery{
		Query: "ray",
		MyID:  1,
	}

	dsl := BuildFunctionScoreDSL("followers_count", q.Map())

	testSearch(t, "users", dsl)
}
