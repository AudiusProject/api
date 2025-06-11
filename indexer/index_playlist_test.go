package indexer

import (
	"testing"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestUpdatePlaylist(t *testing.T) {

	txInfo := TxInfo{}

	err := ci.createPlaylist(txInfo, &core_proto.ManageEntityLegacy{
		EntityId: 1,
		UserId:   1,
		Metadata: toMetadata(map[string]any{
			"playlist_name": "Test Playlist",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from playlists where playlist_name = 'Test Playlist'`)

	err = ci.updatePlaylist(txInfo, &core_proto.ManageEntityLegacy{
		EntityId: 1,
		UserId:   1,
		Metadata: toMetadata(map[string]any{
			"playlist_name": "Test Playlist 2",
			// "description":   "A test playlist description",
			// "cover_art_cid": "QmTestCID",
			// "extra_field":   "extra_value",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from playlists where playlist_name = 'Test Playlist 2'`)
}
