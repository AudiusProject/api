package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// seedCollaborators adds: user 1 accepted on track 700 (owned by user 500) and
// user 1 pending on track 701 (owned by user 500).
func seedCollaborators(t *testing.T, app *ApiServer) {
	// created_at must equal updated_at so the notification trigger's "new pending
	// invite" branch fires (it mirrors the ETL, which writes them equal on
	// insert). The seed baseRow's defaults call time.Now() twice, which only
	// round-trip equal when both calls land in the same microsecond — a flaky
	// coin flip. Pin a single value for both.
	now := time.Now()
	database.SeedTable(app.pool.Replicas[0], "track_collaborators", []map[string]any{
		{"track_id": 700, "collaborator_user_id": 1, "invited_by": 500, "status": "accepted", "created_at": now, "updated_at": now},
		{"track_id": 701, "collaborator_user_id": 1, "invited_by": 500, "status": "pending", "created_at": now, "updated_at": now},
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

// An invited collaborator can see a private (unlisted) track they're on; other
// users cannot. Exercises the get_tracks visibility clause directly.
func TestCollaboratorSeesPrivateTrack(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()

	// Track 700 (owned by user 500) is private/unlisted.
	_, err := app.pool.Replicas[0].Exec(ctx,
		"UPDATE tracks SET is_unlisted = true WHERE track_id = 700 AND is_current = true")
	assert.NoError(t, err)

	// User 1 is a pending collaborator (hasn't accepted yet) — they still need
	// to see the track to decide.
	now := time.Now()
	database.SeedTable(app.pool.Replicas[0], "track_collaborators", []map[string]any{
		{"track_id": 700, "collaborator_user_id": 1, "invited_by": 500, "status": "pending", "created_at": now, "updated_at": now},
	})

	// Collaborator (user 1) sees the private track.
	rows, err := app.queries.GetTracks(ctx, dbv1.GetTracksParams{
		Ids:  []int32{700},
		MyID: int32(1),
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 1, "an invited collaborator should see the private track")

	// A non-collaborator (user 2) does not.
	rows, err = app.queries.GetTracks(ctx, dbv1.GetTracksParams{
		Ids:  []int32{700},
		MyID: int32(2),
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 0, "a non-collaborator must not see the private track")

	// Once the collaborator declines, they no longer see it.
	_, err = app.pool.Replicas[0].Exec(ctx,
		"UPDATE track_collaborators SET status = 'rejected' WHERE track_id = 700 AND collaborator_user_id = 1")
	assert.NoError(t, err)
	rows, err = app.queries.GetTracks(ctx, dbv1.GetTracksParams{
		Ids:  []int32{700},
		MyID: int32(1),
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 0, "a rejected collaborator must not see the private track")
}

// Pending collaborator invites are embedded only on the owner's own tracks (so
// their edit form can preserve them); accepted collaborators stay public.
func TestPendingCollaboratorsVisibleToOwnerOnly(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()

	// Track 700 is owned by user 500: user 1 accepted, user 2 still pending.
	now := time.Now()
	database.SeedTable(app.pool.Replicas[0], "track_collaborators", []map[string]any{
		{"track_id": 700, "collaborator_user_id": 1, "invited_by": 500, "status": "accepted", "created_at": now, "updated_at": now},
		{"track_id": 700, "collaborator_user_id": 2, "invited_by": 500, "status": "pending", "created_at": now, "updated_at": now},
	})

	// As the owner (my_id = 500): accepted embedded + pending visible.
	owned, err := app.queries.TracksKeyed(ctx, dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{Ids: []int32{700}, MyID: int32(500)},
	})
	assert.NoError(t, err)
	assert.Len(t, owned[700].Collaborators, 1, "accepted collaborator is embedded")
	assert.Len(t, owned[700].PendingCollaborators, 1, "owner sees the pending invite")

	// As a non-owner (my_id = 1): accepted still embedded, pending hidden.
	other, err := app.queries.TracksKeyed(ctx, dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{Ids: []int32{700}, MyID: int32(1)},
	})
	assert.NoError(t, err)
	assert.Len(t, other[700].Collaborators, 1, "accepted collaborator stays public")
	assert.Len(t, other[700].PendingCollaborators, 0, "pending invite is hidden from non-owners")
}
