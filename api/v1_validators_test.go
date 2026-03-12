package api

import (
	"testing"

	"api.audius.co/config"
	"github.com/stretchr/testify/assert"
)

func TestNodesSetGet(t *testing.T) {
	nodesStore := NewNodes()

	nodes := []config.Node{
		{Id: "1", Endpoint: "https://validator-1.test"},
		{Id: "2", Endpoint: "https://validator-2.test"},
	}

	nodesStore.SetNodes(nodes)

	got := nodesStore.GetNodes()
	assert.Equal(t, nodes, got)

	// Ensure GetNodes returns a copy and not the underlying slice.
	got[0].Endpoint = "https://changed.test"
	gotAgain := nodesStore.GetNodes()
	assert.Equal(t, "https://validator-1.test", gotAgain[0].Endpoint)
}

func TestV1Validators(t *testing.T) {
	app := emptyTestApp(t)

	app.validators.SetNodes([]config.Node{
		{Id: "1", Endpoint: "https://validator-1.test"},
		{Id: "2", Endpoint: "https://validator-2.test"},
	})

	status, body := testGet(t, app, "/v1/validators")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"nodes.#":          2,
		"nodes.0.Endpoint": "https://validator-1.test",
		"nodes.1.Endpoint": "https://validator-2.test",
	})
}
