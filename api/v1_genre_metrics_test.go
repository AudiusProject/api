package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenreMetrics(t *testing.T) {
	// Calculate timestamp for 1 hour ago
	oneHourAgo := time.Now().Add(-1 * time.Hour).Unix()
	url := fmt.Sprintf("/v1/metrics/genres?start_time=%d", oneHourAgo)

	var response struct {
		Data []struct {
			Genre string `json:"genre"`
			Count int    `json:"count"`
		}
	}

	status, _ := testGet(t, url, &response)
	assert.Equal(t, 200, status)

	// Find the Electronic genre in the response
	for _, genre := range response.Data {
		if genre.Genre == "Electronic" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Electronic track")
			break
		}
		if genre.Genre == "Alternative" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Alternative track")
			break
		}
		if genre.Genre == "Folk" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Folk track")
			break
		}
	}

}
