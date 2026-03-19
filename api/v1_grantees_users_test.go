package api

import (
	"strings"
	"testing"

	"api.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

// grantee address 0x681c616ae836ceca1effe00bd07f2fdbf9a082bc is the wallet of user 100 (authtest1).
// Grants for this address:
//   user_id=1, is_approved=false, is_revoked=false
//   user_id=2, is_approved=true,  is_revoked=false
//   user_id=3, is_approved=true,  is_revoked=true
//   user_id=4, is_approved=false, is_revoked=true

const testGranteeAddress = "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc"

// Default params: is_revoked=false, all approval statuses
func TestGetGranteeUsersNoParams(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, _ := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(response.Data))
}

// Only approved grants (is_revoked defaults to false)
func TestGetGranteeUsersApproved(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, body := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?is_approved=true", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(response.Data))
	jsonAssert(t, body, map[string]any{
		"data.0.handle": "stereosteve",
	})
}

// Only unapproved grants (is_revoked defaults to false)
func TestGetGranteeUsersNotApproved(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, body := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?is_approved=false", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(response.Data))
	jsonAssert(t, body, map[string]any{
		"data.0.handle": "rayjacobson",
	})
}

// Revoked grants
func TestGetGranteeUsersRevoked(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, _ := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?is_revoked=true", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(response.Data))
}

// Address without 0x prefix should work the same as with prefix
func TestGetGranteeUsersWithoutHexPrefix(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	addressWithoutPrefix := strings.TrimPrefix(testGranteeAddress, "0x")
	status, _ := testGet(t, app, "/v1/grantees/"+addressWithoutPrefix+"/users", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(response.Data))
}

// Grantee address with no grants returns empty list
func TestGetGranteeUsersNotFound(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, _ := testGet(t, app, "/v1/grantees/0xdeadbeef/users", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, len(response.Data))
}

// Pagination: limit=1 should return only 1 user
func TestGetGranteeUsersPagination(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}
	status, _ := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?limit=1", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(response.Data))
}

// Invalid param values should return 400
func TestGetGranteeUsersInvalidParams(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []dbv1.User
	}

	status, _ := testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?is_approved=invalid", &response)
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/grantees/"+testGranteeAddress+"/users?is_revoked=invalid", &response)
	assert.Equal(t, 400, status)
}
