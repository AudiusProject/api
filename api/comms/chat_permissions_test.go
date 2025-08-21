package comms

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatPermissions(t *testing.T) {
	// Create test database
	pool := database.CreateTestDatabase(t, "test_api")
	defer pool.Close()

	ctx := context.Background()

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	user1Id := seededRand.Int31()
	user2Id := seededRand.Int31()
	user3Id := seededRand.Int31()

	// Set up test data and commit it so validator can see it
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		// user 1 follows user 2
		_, err = tx.Exec(ctx, "insert into follows (follower_user_id, followee_user_id, is_current, is_delete, created_at) values ($1, $2, true, false, now())", user1Id, user2Id)
		require.NoError(t, err)
		// user 3 has tipped user 1
		_, err = tx.Exec(ctx, `
		insert into user_tips
			(slot, signature, sender_user_id, receiver_user_id, amount, created_at, updated_at)
		values
			(1, 'c', $1, $2, 100, now(), now())
		`, user3Id, user1Id)
		require.NoError(t, err)

		// Commit the test data so validator can see it
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}

	// Create validator for validation testing
	validator := CreateTestValidator(t, pool)

	assertPermissionValidation := func(sender int32, receiver int32, errorExpected bool) {
		assertChatCreateAllowed(t, ctx, validator, sender, receiver, !errorExpected)
	}

	// validate user1Id can set permissions
	{
		exampleRpc := RawRPC{
			Params: fmt.Appendf(nil, `{"permit": "all"}`),
		}

		err := validator.validateChatPermit(user1Id, exampleRpc)
		assert.NoError(t, err)
	}

	// user 1 sets chat permissions to followees only
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		err = chatSetPermissions(tx, ctx, int32(user1Id), ChatPermissionFollowees, nil, nil, time.Now())
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}
	// user 2 can message user 1 since 1 follows 2
	assertPermissionValidation(user2Id, user1Id, false)
	// user 3 cannot message user 1 since 1 does not follow 3
	assertPermissionValidation(user3Id, user1Id, true)

	// user 1 sets chat permissions to tippers only
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		err = chatSetPermissions(tx, ctx, int32(user1Id), ChatPermissionTippers, nil, nil, time.Now())
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}
	// user 2 cannot message user 1 since 2 has never tipped 1
	assertPermissionValidation(user2Id, user1Id, true)
	// user 3 can message user 1 since 3 has tipped 1
	assertPermissionValidation(user3Id, user1Id, false)

	// user 1 changes chat permissions to none
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		err = chatSetPermissions(tx, ctx, int32(user1Id), ChatPermissionNone, nil, nil, time.Now())
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}
	// no one can message user 1
	assertPermissionValidation(user2Id, user1Id, true)
	assertPermissionValidation(user3Id, user1Id, true)

	// user 1 changes chat permissions to all
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		err = chatSetPermissions(tx, ctx, int32(user1Id), ChatPermissionAll, nil, nil, time.Now())
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}

	// meanwhile... a "late" permission update is ignored
	// the "all" setting should prevail
	{
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		err = chatSetPermissions(tx, ctx, int32(user1Id), ChatPermissionNone, nil, nil, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)
	}

	// anyone can message user 1
	assertPermissionValidation(user2Id, user1Id, false)
	assertPermissionValidation(user3Id, user1Id, false)
}

func assertChatCreateAllowed(t *testing.T, ctx context.Context, validator *Validator, sender int32, receiver int32, shouldWork bool) {
	chatId := trashid.ChatID(int(sender), int(receiver))
	exampleRpc := RawRPC{
		Params: []byte(fmt.Sprintf(`{"chat_id": "%s", "invites": [{"user_id": "%s", "invite_code": "%s"}, {"user_id": "%s", "invite_code": "%s"}]}`, chatId, trashid.MustEncodeHashID(int(sender)), trashid.MustEncodeHashID(int(sender)), trashid.MustEncodeHashID(int(receiver)), trashid.MustEncodeHashID(int(sender)))),
	}
	err := validator.validateChatCreate(ctx, sender, exampleRpc)
	if shouldWork {
		assert.NoError(t, err)
	} else {
		assert.ErrorContains(t, err, "Not permitted to send messages to this user")
	}
}
