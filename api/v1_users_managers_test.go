package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

// Defaults to all approval status and no revoked managers
func TestGetUsersManagersNoParams(t *testing.T) {
	app := testAppWithFixtures(t)
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, body := testGet(t, app, "/v1/users/7eP5n/managers", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managersResponse.Data))

	jsonAssert(t, body, map[string]any{
		"data.0.manager.wallet":    "0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8",
		"data.0.grant.is_approved": true,
		"data.1.manager.wallet":    "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc",
		"data.1.grant.is_approved": false,
	})
}

// Should return only approved managers and default to not showing revoked managers
func TestGetUsersManagersApproved(t *testing.T) {
	app := testAppWithFixtures(t)
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, body := testGet(t, app, "/v1/users/7eP5n/managers?is_approved=true", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(managersResponse.Data))

	jsonAssert(t, body, map[string]any{
		"data.0.manager.wallet":    "0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8",
		"data.0.grant.is_approved": true,
	})
}

func TestGetUsersManagersRevoked(t *testing.T) {
	app := testAppWithFixtures(t)
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, body := testGet(t, app, "/v1/users/7eP5n/managers?is_revoked=true", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managersResponse.Data))

	jsonAssert(t, body, map[string]any{
		"data.0.manager.wallet":    "0xc451c1f8943b575158310552b41230c61844a1c1",
		"data.0.grant.is_approved": false,
		"data.0.grant.is_revoked":  true,
		"data.1.manager.wallet":    "0x1234567890abcdef",
		"data.1.grant.is_approved": true,
		"data.1.grant.is_revoked":  true,
	})
}

func TestGetUsersManagersInvalidParams(t *testing.T) {
	app := testAppWithFixtures(t)
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, _ := testGet(t, app, "/v1/users/7eP5n/managers?is_approved=invalid", &managersResponse)
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, "/v1/users/7eP5n/managers?is_revoked=invalid", &managersResponse)
	assert.Equal(t, 400, status)
}
