package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestV1CoinsVolumeLeaders(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/coins/volume-leaders?from=not-a-date")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "from is invalid",
	})

	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-2 * time.Hour).Format(time.RFC3339)
	to := now.Add(-1 * time.Hour).Format(time.RFC3339)

	status, body = testGet(
		t, app,
		fmt.Sprintf("/v1/coins/volume-leaders?from=%s&to=%s&limit=5&offset=0", from, to),
	)
	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "data").IsArray())

	status, body = testGet(
		t, app,
		fmt.Sprintf("/v1/coins/volume-leaders?from=%s&to=%s", now.Format(time.RFC3339), now.Add(-1*time.Hour).Format(time.RFC3339)),
	)
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "To date must be after from date",
	})

	status, body = testGet(
		t, app,
		fmt.Sprintf("/v1/coins/volume-leaders?from=%s&to=%s", now.Add(-9*24*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339)),
	)
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Time range must be <= 7 days",
	})

	tooOldFrom := now.Add(-12 * 24 * time.Hour)
	tooOldTo := tooOldFrom.Add(6 * time.Hour)
	status, body = testGet(
		t, app,
		fmt.Sprintf("/v1/coins/volume-leaders?from=%s&to=%s", tooOldFrom.Format(time.RFC3339), tooOldTo.Format(time.RFC3339)),
	)
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{
		"error": "Time range too old",
	})
}
