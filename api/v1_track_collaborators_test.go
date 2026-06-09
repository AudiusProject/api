package api

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// seedCollaborators adds: user 1 accepted on track 700 (owned by user 500) and
// user 1 pending on track 701 (owned by user 500).
func seedCollaborators(t *testing.T, app *ApiServer) {
	database.SeedTable(app.pool.Replicas[0], "track_collaborators", []map[string]any{
		{"track_id": 700, "collaborator_user_id": 1, "invited_by": 500, "status": "accepted"},
		{"track_id": 701, "collaborator_user_id": 1, "invited_by": 500, "status": "pending"},
	})
}

// Accepted collaborators are embedded on the track response.
func TestTrackCollaboratorsEmbeddedOnTrack(t *testing.T) {
	app := testAppWithFixtures(t)
	seedCollaborators(t, app)

	var resp struct {
		Data []dbv1.Track
	}
	// User 500's tracks default-sort to 701, 703, 702, 700 — so 700 is index 3.
	status, body := testGet(t, app, fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500)), &resp)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.3.id":                     trashid.MustEncodeHashID(700),
		"data.3.collaborators.0.handle": "rayjacobson",
	})
	// Non-collaborated tracks carry an empty array, not null.
	assert.Contains(t, string(body), `"collaborators":[]`)
}

// An accepted collaboration surfaces the track on the collaborator's profile.
func TestAcceptedCollaborationAppearsOnProfile(t *testing.T) {
	app := testAppWithFixtures(t)
	seedCollaborators(t, app)

	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(1)), &resp)
	assert.Equal(t, 200, status)

	found := false
	for _, track := range resp.Data {
		if track.ID == trashid.MustEncodeHashID(700) {
			found = true
		}
	}
	assert.True(t, found, "accepted collaboration (track 700) should appear on user 1's profile")
}

// A pending invite is excluded from the profile (only accepted credits surface).
func TestPendingCollaborationHiddenFromProfile(t *testing.T) {
	app := testAppWithFixtures(t)
	seedCollaborators(t, app)

	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(1)), &resp)
	assert.Equal(t, 200, status)

	for _, track := range resp.Data {
		assert.NotEqual(t, trashid.MustEncodeHashID(701), track.ID, "pending invite (track 701) must not appear on profile")
	}
}

func TestCollaborationInvitesEndpoint(t *testing.T) {
	app := testAppWithFixtures(t)
	seedCollaborators(t, app)

	var resp struct {
		Data []dbv1.FullTrackCollaboratorInvite
	}

	// Pending only.
	status, body := testGet(t, app, fmt.Sprintf("/v1/users/%s/collaboration_invites?status=pending", trashid.MustEncodeHashID(1)), &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	jsonAssert(t, body, map[string]any{
		"data.0.track_id":          trashid.MustEncodeHashID(701),
		"data.0.status":            "pending",
		"data.0.invited_by.handle": "UserTracksTester",
	})

	// No filter returns both the pending and accepted credits.
	status, _ = testGet(t, app, fmt.Sprintf("/v1/users/%s/collaboration_invites", trashid.MustEncodeHashID(1)), &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(resp.Data))

	// Invalid status is rejected.
	status, _ = testGet(t, app, fmt.Sprintf("/v1/users/%s/collaboration_invites?status=bogus", trashid.MustEncodeHashID(1)), &resp)
	assert.Equal(t, 400, status)
}

// Seeding rows fires the notification trigger: an invite notifies the
// collaborator, an accept notifies the inviter.
func TestTrackCollaboratorNotificationsGenerated(t *testing.T) {
	app := testAppWithFixtures(t)
	seedCollaborators(t, app)

	var inviteCount int
	err := app.pool.Replicas[0].QueryRow(context.Background(),
		"SELECT count(*) FROM notification WHERE type = 'track_collaborator_invite' AND 1 = ANY(user_ids)").Scan(&inviteCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, inviteCount, "pending invite should notify the collaborator (user 1)")

	var acceptCount int
	err = app.pool.Replicas[0].QueryRow(context.Background(),
		"SELECT count(*) FROM notification WHERE type = 'track_collaborator_accept' AND 500 = ANY(user_ids)").Scan(&acceptCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, acceptCount, "accepted credit should notify the inviter (user 500)")
}
