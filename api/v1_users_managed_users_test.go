package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// Defaults to all approval status and no revoked managers
func TestGetManagedUsersNoParams(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, body := testGet(t, "/v1/users/eYZmn/managed_users", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managedUsersResponse.Data))

	jsonAssert(t, body, map[string]string{
		"data.0.user.id":           trashid.MustEncodeHashID(1),
		"data.0.grant.is_approved": "false",
		"data.1.user.id":           trashid.MustEncodeHashID(2),
		"data.1.grant.is_approved": "true",
	})
}

// Should return only approved managers and default to not showing revoked managers
func TestGetManagedUsersApproved(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, body := testGet(t, "/v1/users/eYZmn/managed_users?is_approved=true", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(managedUsersResponse.Data))

	jsonAssert(t, body, map[string]string{
		"data.0.user.id":           trashid.MustEncodeHashID(2),
		"data.0.grant.is_approved": "true",
	})
}

func TestGetManagedUsersRevoked(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, body := testGet(t, "/v1/users/eYZmn/managed_users?is_revoked=true", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managedUsersResponse.Data))

	jsonAssert(t, body, map[string]string{
		"data.0.user.id":           trashid.MustEncodeHashID(3),
		"data.0.grant.is_approved": "true",
		"data.0.grant.is_revoked":  "true",
		"data.1.user.id":           trashid.MustEncodeHashID(4),
		"data.1.grant.is_approved": "false",
		"data.1.grant.is_revoked":  "true",
	})
}

func TestGetManagedUsersInvalidParams(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, _ := testGet(t, "/v1/users/eYZmn/managed_users?is_approved=invalid", &managedUsersResponse)
	assert.Equal(t, 400, status)

	status, _ = testGet(t, "/v1/users/eYZmn/managed_users?is_revoked=invalid", &managedUsersResponse)
	assert.Equal(t, 400, status)
}
