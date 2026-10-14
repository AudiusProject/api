package api

import (
	"api.audius.co/database"
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersAccount(t *testing.T) {
	app := testAppWithFixtures(t)
	var accountResponse struct {
		Data dbv1.FullAccount
	}
	status, _ := testGetWithWallet(t, app, "/v1/users/account/0x7d273271690538cf855e5b3002a0dd8c154bb060", "0x7d273271690538cf855e5b3002a0dd8c154bb060", &accountResponse)
	assert.Equal(t, 200, status)

	assert.Equal(t, "0x7d273271690538cf855e5b3002a0dd8c154bb060", accountResponse.Data.User.Wallet.String)
	// User 1 has three track saves in SaveFixtures: 100 (live), 950 (deleted
	// track), 951 (deactivated owner). TrackSaveCount comes from
	// aggregate_user (raw count, unfiltered) so it sees all three.
	assert.Equal(t, (int64)(3), accountResponse.Data.TrackSaveCount)

	// Check playlists
	assert.Equal(t, 3, len(accountResponse.Data.Playlists))
	assert.Equal(t, "First", accountResponse.Data.Playlists[0].Name)
	assert.Equal(t, false, accountResponse.Data.Playlists[0].IsAlbum)
	assert.Equal(t, "rayjacobson", accountResponse.Data.User.Handle.String)

	assert.Equal(t, "SecondAlbum", accountResponse.Data.Playlists[2].Name)
	assert.Equal(t, true, accountResponse.Data.Playlists[2].IsAlbum)
	assert.Equal(t, "rayjacobson", accountResponse.Data.User.Handle.String)

	// Check playlist library
	assert.Equal(t, 3, len(accountResponse.Data.PlaylistLibrary.Contents))

	// Check first item (regular playlist)
	playlist, ok := accountResponse.Data.PlaylistLibrary.Contents[0].(dbv1.RegularPlaylist)
	assert.True(t, ok)
	assert.Equal(t, "playlist", playlist.Type)
	assert.Equal(t, 123, int(playlist.PlaylistID))

	// Check second item (explore playlist)
	explorePlaylist, ok := accountResponse.Data.PlaylistLibrary.Contents[1].(dbv1.ExplorePlaylist)
	assert.True(t, ok)
	assert.Equal(t, "explore_playlist", explorePlaylist.Type)
	assert.Equal(t, "Audio NFTs", explorePlaylist.PlaylistID)

	// Check third item (folder)
	folder, ok := accountResponse.Data.PlaylistLibrary.Contents[2].(dbv1.Folder)
	assert.True(t, ok)
	assert.Equal(t, "folder", folder.Type)
	assert.Equal(t, "bbcae31a-7cd2-4a1a-8b54-fdc979a34435", folder.ID)
	assert.Equal(t, "My Nested Playlists", folder.Name)
	assert.Equal(t, 1, len(folder.Contents))

	// Check nested playlist in folder
	nestedPlaylist, ok := folder.Contents[0].(dbv1.RegularPlaylist)
	assert.True(t, ok)
	assert.Equal(t, "playlist", nestedPlaylist.Type)
	assert.Equal(t, 345, int(nestedPlaylist.PlaylistID))
}

// Wallets can have an old deactivated account or an unfinished signup as well
// as the active profile. Resolve the active, completed profile consistently.
func TestGetUsersAccountSharedWallet(t *testing.T) {
	for _, tc := range []struct {
		name        string
		users       []map[string]any
		want        int32
		deactivated bool
	}{
		{name: "active profile over older deactivated profile", users: []map[string]any{
			{"user_id": 1, "handle": "old", "is_deactivated": true, "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
			{"user_id": 2, "handle": "active", "created_at": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)},
		}, want: 2},
		{name: "completed profile over unfinished signup", users: []map[string]any{
			{"user_id": 1, "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
			{"user_id": 2, "handle": "active", "created_at": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)},
		}, want: 2},
		{name: "deactivated-only wallet remains deactivated", users: []map[string]any{
			{"user_id": 1, "handle": "old", "is_deactivated": true},
		}, want: 1, deactivated: true},
		{name: "unfinished signup remains available", users: []map[string]any{
			{"user_id": 1},
		}, want: 1},
		{name: "oldest completed active profile wins", users: []map[string]any{
			{"user_id": 1, "handle": "first", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
			{"user_id": 2, "handle": "second", "created_at": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)},
		}, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := emptyTestApp(t)
			const wallet = "0x7d273271690538cf855e5b3002a0dd8c154bb060"
			for _, u := range tc.users {
				u["wallet"] = wallet
			}
			database.Seed(app.pool.Replicas[0], database.FixtureMap{"users": tc.users})
			resolved, err := app.queries.GetUserForWallet(context.Background(), wallet)
			require.NoError(t, err)
			assert.Equal(t, tc.want, int32(resolved))
			var response struct{ Data dbv1.FullAccount }
			status, _ := testGetWithWallet(t, app, "/v1/users/account/"+wallet, wallet, &response)
			require.Equal(t, 200, status)
			assert.Equal(t, tc.want, response.Data.User.UserID)
			assert.Equal(t, tc.deactivated, response.Data.User.IsDeactivated)
		})
	}
}
