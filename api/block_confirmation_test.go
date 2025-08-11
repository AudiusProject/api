package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestBlockConfirmation(t *testing.T) {
	app := emptyTestApp(t)

	// Create a block confirmation fixture
	fixtures := database.FixtureMap{
		"core_indexed_blocks": {
			{
				"height":    1,
				"chain_id":  "audius-mainnet-alpha-beta",
				"blockhash": "0xabc123",
			},
			{
				"height":    2,
				"chain_id":  "audius-mainnet-alpha-beta",
				"blockhash": "0xabc234",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/block_confirmation?blockhash=0x123&blocknumber=3000")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.block_passed": false,
		"data.block_found":  false,
	})

	statusFound, bodyFound := testGet(t, app, "/block_confirmation?blockhash=0xabc123&blocknumber=1")
	assert.Equal(t, 200, statusFound)
	jsonAssert(t, bodyFound, map[string]any{
		"data.block_passed": true,
		"data.block_found":  true,
	})
}
