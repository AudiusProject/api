package api

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUserChallengesRetired(t *testing.T) {
	app := testAppWithFixtures(t)
	for _, user := range []string{"eYVJn", "eP7kD", "L50xn"} {
		status, body := testGet(t, app, "/v1/users/"+user+"/challenges")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.#": float64(0)})
	}
}
