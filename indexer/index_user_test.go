package indexer

import (
	"testing"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestUser(t *testing.T) {
	txInfo := TxInfo{}
	err := ci.createUser(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   33,
		EntityId: 33,
		Metadata: toMetadata(map[string]any{
			"handle": "test33",
			"name":   "test33_before",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from users where name = 'test33_before'`)

	err = ci.updateUser(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   33,
		EntityId: 33,
		Metadata: toMetadata(map[string]any{
			"handle": "test33_after", // should ignore handle edits
			"name":   "test33_after",
		}),
	})
	assert.NoError(t, err)
	assertCount(t, 1, `select count(*) from users where name = 'test33_after'`)

	// handle edit ignored
	assertCount(t, 0, `select count(*) from users where handle = 'test33_after'`)
	assertCount(t, 1, `select count(*) from users where handle = 'test33'`)

}
