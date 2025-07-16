package indexer

import (
	"testing"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestIndexTrack(t *testing.T) {
	var txInfo TxInfo

	// CREATE
	err := ci.createTrack(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   51,
		EntityId: 51,
		Metadata: toMetadata(map[string]any{
			"title": "track51",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from tracks where title = 'track51'`)

	// UPDATE
	err = ci.updateTrack(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   51,
		EntityId: 51,
		Metadata: toMetadata(map[string]any{
			"title": "track51 update",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from tracks where title = 'track51 update'`)

	// DELETE
	err = ci.deleteTrack(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   51,
		EntityId: 51,
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from tracks where title = 'track51 update' and is_delete = true`)
}
