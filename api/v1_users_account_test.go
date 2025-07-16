package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
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
	assert.Equal(t, (int64)(1), accountResponse.Data.TrackSaveCount)

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
