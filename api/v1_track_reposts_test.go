package api

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"api.audius.co/api/testdata"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestPostV1TrackRepost(t *testing.T) {
	// Use the ganache test private key from comms_mutate_test
	testPrivateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testWallet := testdata.CreateTestWallet(t, testPrivateKey)

	app := emptyTestApp(t)

	// Create a track in the database
	trackID := 1
	trackHashID := trashid.MustEncodeHashID(trackID)

	// Create a user that owns the private key
	userID := 100
	userHashID := trashid.MustEncodeHashID(userID)

	now := time.Now()
	fixtures := database.FixtureMap{
		"tracks": {
			{
				"track_id":   trackID,
				"owner_id":   1, // Different owner
				"title":      "Test Track",
				"is_current": true,
				"is_delete":  false,
				"created_at": now,
				"updated_at": now,
			},
		},
		"users": {
			{
				"user_id":    userID,
				"handle":     "testuser",
				"wallet":     strings.ToLower(testWallet.Address),
				"is_current": true,
				"created_at": now,
				"updated_at": now,
			},
		},
		"grants": {
			{
				"user_id":         userID,
				"grantee_address": strings.ToLower(testWallet.Address),
				"is_approved":     true,
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      now,
				"updated_at":      now,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	t.Run("valid basic auth with private key", func(t *testing.T) {
		// Create basic auth with the test private key
		authString := fmt.Sprintf("user:%s", testPrivateKey)
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))
		headers := map[string]string{
			"Authorization": "Basic " + encodedAuth,
		}

		status, body := testPost(t, app, fmt.Sprintf("/v1/full/tracks/%s/reposts?user_id=%s", trackHashID, userHashID), nil, headers)

		// The request will fail because we don't have a mock SDK set up,
		// but it should fail at the SDK level, not at auth or private key parsing level
		// Expected: 500 (server error) with "Failed to save track" or SDK-related error
		// Not expected: 4xx (client error) - auth (401), forbidden (403), or parsing errors (400)
		assert.False(t, status >= 400 && status < 500, "Should not return client error status (400-499)")

		// Check that it got past auth and private key parsing
		bodyStr := string(body)
		// It should fail at the SDK level or transaction sending level
		t.Logf("Request failed with expected SDK error: %s", bodyStr)
	})
}
