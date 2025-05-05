package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

// Defaults to all approval status and no revoked managers
func TestGetUsersManagersNoParams(t *testing.T) {
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, _ := testGet(t, "/v1/users/7eP5n/managers", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managersResponse.Data))

	assert.Equal(t, "0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8", managersResponse.Data[0].Manager.Wallet.String)
	assert.Equal(t, true, managersResponse.Data[0].Grant.IsApproved.Bool)
	assert.Equal(t, "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc", managersResponse.Data[1].Manager.Wallet.String)
	assert.Equal(t, false, managersResponse.Data[1].Grant.IsApproved.Bool)
}

// Should return only approved managers and default to not showing revoked managers
func TestGetUsersManagersApproved(t *testing.T) {
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, _ := testGet(t, "/v1/users/7eP5n/managers?is_approved=true", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managersResponse.Data))
	assert.Equal(t, "0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8", managersResponse.Data[0].Manager.Wallet.String)
	assert.Equal(t, true, managersResponse.Data[0].Grant.IsApproved.Bool)
}

func TestGetUsersManagersRevoked(t *testing.T) {
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, _ := testGet(t, "/v1/users/7eP5n/managers?is_revoked=true", &managersResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(managersResponse.Data))
	assert.Equal(t, "0xc451c1f8943b575158310552b41230c61844a1c1", managersResponse.Data[0].Manager.Wallet.String)
	assert.Equal(t, false, managersResponse.Data[0].Grant.IsApproved.Bool)
	assert.Equal(t, true, managersResponse.Data[0].Grant.IsRevoked)
	assert.Equal(t, "0x1234567890abcdef", managersResponse.Data[1].Manager.Wallet.String)
	assert.Equal(t, true, managersResponse.Data[1].Grant.IsApproved.Bool)
	assert.Equal(t, true, managersResponse.Data[1].Grant.IsRevoked)
}

func TestInvalidParams(t *testing.T) {
	var managersResponse struct {
		Data []dbv1.FullManager
	}
	status, _ := testGet(t, "/v1/users/7eP5n/managers?is_approved=invalid", &managersResponse)
	assert.Equal(t, 400, status)

	status, _ = testGet(t, "/v1/users/7eP5n/managers?is_revoked=invalid", &managersResponse)
	assert.Equal(t, 400, status)
}
