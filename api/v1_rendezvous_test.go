package api

import (
	"testing"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestV1Rendezvous(t *testing.T) {
	app := emptyTestApp(t)

	nodes := []config.Node{
		{Endpoint: "https://validator-1.test"},
		{Endpoint: "https://validator-2.test"},
		{Endpoint: "https://validator-3.test"},
	}
	app.validators.SetNodes(nodes)
	rendezvous.Refresh(nodes)

	status, body := testGet(t, app, "/v1/rendezvous/bafybeigdyr")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"validators.#": 3,
	})
	assert.True(t, gjson.GetBytes(body, "first").Exists())
	assert.True(t, gjson.GetBytes(body, "rest").IsArray())
}
