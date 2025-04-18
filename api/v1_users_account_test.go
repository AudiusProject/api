package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersAccount(t *testing.T) {
	var accountResponse struct {
		Data dbv1.FullAccount
	}
	status, _ := testGetWithWallet(t, "/v1/users/account/0x7d273271690538cf855e5b3002a0dd8c154bb060", "0x7d273271690538cf855e5b3002a0dd8c154bb060", &accountResponse)
	assert.Equal(t, 200, status)

	assert.Equal(t, "0x7d273271690538cf855e5b3002a0dd8c154bb060", accountResponse.Data.User.Wallet.String)
	assert.Equal(t, (int64)(20), accountResponse.Data.TrackSaveCount)

	// Check playlists
	assert.Equal(t, 2, len(accountResponse.Data.Playlists))
	assert.Equal(t, "SecondAlbum", accountResponse.Data.Playlists[0].Name)
	assert.Equal(t, true, accountResponse.Data.Playlists[0].IsAlbum)
	assert.Equal(t, "rayjacobson", accountResponse.Data.User.Handle.String)
}
