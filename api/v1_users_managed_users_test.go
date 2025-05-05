package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

// Defaults to all approval status and no revoked managers
func TestGetManagedUsersNoParams(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, _ := testGet(t, "/v1/users/eYZmn/managed_users", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managedUsersResponse.Data))

	assert.Equal(t, 1, int(managedUsersResponse.Data[0].User.ID))
	assert.Equal(t, false, managedUsersResponse.Data[0].Grant.IsApproved.Bool)
	assert.Equal(t, 2, int(managedUsersResponse.Data[1].User.ID))
	assert.Equal(t, true, managedUsersResponse.Data[1].Grant.IsApproved.Bool)
}

// Should return only approved managers and default to not showing revoked managers
func TestGetManagedUsersApproved(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, _ := testGet(t, "/v1/users/eYZmn/managed_users?is_approved=true", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(managedUsersResponse.Data))
	assert.Equal(t, 2, int(managedUsersResponse.Data[0].User.ID))
	assert.Equal(t, true, managedUsersResponse.Data[0].Grant.IsApproved.Bool)
}

func TestGetManagedUsersRevoked(t *testing.T) {
	var managedUsersResponse struct {
		Data []dbv1.FullManagedUser
	}
	status, _ := testGet(t, "/v1/users/eYZmn/managed_users?is_revoked=true", &managedUsersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managedUsersResponse.Data))
	assert.Equal(t, 3, int(managedUsersResponse.Data[0].User.ID))
	assert.Equal(t, true, managedUsersResponse.Data[0].Grant.IsApproved.Bool)
	assert.Equal(t, true, managedUsersResponse.Data[0].Grant.IsRevoked)
	assert.Equal(t, 4, int(managedUsersResponse.Data[1].User.ID))
	assert.Equal(t, false, managedUsersResponse.Data[1].Grant.IsApproved.Bool)
	assert.Equal(t, true, managedUsersResponse.Data[1].Grant.IsRevoked)
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
