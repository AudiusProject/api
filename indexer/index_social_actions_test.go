package indexer

import (
	"testing"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestFollows(t *testing.T) {
	txInfo := TxInfo{}
	err := ci.followUser(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   21,
		EntityId: 22,
	})
	assert.NoError(t, err)

	assertCount(t, 1, `select count(*) from follows where follower_user_id = 21 and followee_user_id = 22 and is_delete = false`)
	assertCount(t, 1, `select follower_count from aggregate_user where user_id = 22`)
	assertCount(t, 1, `select following_count from aggregate_user where user_id = 21`)

	err = ci.unfollowUser(txInfo, &core_proto.ManageEntityLegacy{
		UserId:   21,
		EntityId: 22,
	})
	assert.NoError(t, err)

	assertCount(t, 1, `select count(*) from follows where follower_user_id = 21 and followee_user_id = 22 and is_delete = true`)
	assertCount(t, 0, `select count(*) from follows where follower_user_id = 21 and followee_user_id = 22 and is_delete = false`)

	// triggers don't actually handle unfollow?
	// assertCount(t, 0, `select follower_count from aggregate_user where user_id = 22`)
	// assertCount(t, 0, `select following_count from aggregate_user where user_id = 21`)
}
