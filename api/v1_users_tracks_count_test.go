package api

import (
	"fmt"
	"testing"

	"api.audius.co/trashid"

	"api.audius.co/database"

	"github.com/stretchr/testify/assert"
)

func TestGetUserTracksCount(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := testTrackGateFixtures()
	database.Seed(app.pool.Replicas[0], fixtures)

	var response struct {
		Data int
	}

	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks/count", trashid.MustEncodeHashID(600))

	// Test without filter - should return all tracks
	status, _ := testGet(t, app, baseUrl, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 6, response.Data)

	// Test filter for ungated tracks only
	url := fmt.Sprintf("%s?gate_condition=ungated", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test filter for usdc_purchase gated tracks
	url = fmt.Sprintf("%s?gate_condition=usdc_purchase", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test filter for follow gated tracks
	url = fmt.Sprintf("%s?gate_condition=follow", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test filter for tip gated tracks
	url = fmt.Sprintf("%s?gate_condition=tip", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test filter for nft gated tracks
	url = fmt.Sprintf("%s?gate_condition=nft", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test filter for token gated tracks
	url = fmt.Sprintf("%s?gate_condition=token", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)

	// Test multiple gate conditions
	url = fmt.Sprintf("%s?gate_condition=token&gate_condition=usdc_purchase", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, response.Data)

	// Test all exclusive tracks (token + usdc_purchase + follow + tip + nft)
	url = fmt.Sprintf("%s?gate_condition=token&gate_condition=usdc_purchase&gate_condition=follow&gate_condition=tip&gate_condition=nft", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 5, response.Data)
}

func TestGetUserTracksCountWithFilterTracks(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := testTrackGateFixtures()
	database.Seed(app.pool.Replicas[0], fixtures)

	var response struct {
		Data int
	}

	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks/count", trashid.MustEncodeHashID(600))

	// Test with public filter
	url := fmt.Sprintf("%s?filter_tracks=public", baseUrl)
	status, _ := testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	// Should return all public tracks (6 tracks are public by default in fixtures)
	assert.Equal(t, 6, response.Data)

	// Test with public filter and gate condition
	url = fmt.Sprintf("%s?filter_tracks=public&gate_condition=token", baseUrl)
	status, _ = testGet(t, app, url, &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, response.Data)
}

func TestGetUserTracksCountInvalidParams(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := testTrackGateFixtures()
	database.Seed(app.pool.Replicas[0], fixtures)

	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks/count", trashid.MustEncodeHashID(600))

	// Test invalid filter_tracks value
	url := fmt.Sprintf("%s?filter_tracks=invalid", baseUrl)
	status, _ := testGet(t, app, url)
	assert.Equal(t, 400, status)
}
