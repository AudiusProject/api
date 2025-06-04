package api

import (
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserConnectedWalletsQuery(t *testing.T) {
	app := testAppWithFixtures(t)
	connectedWallets, err := app.queries.FullConnectedWallets(t.Context(), 2)
	assert.NoError(t, err)
	assert.Len(t, connectedWallets.ErcWallets, 2)
	assert.Len(t, connectedWallets.SplWallets, 2)
	assert.Contains(t, connectedWallets.ErcWallets, "0x1111111111111111111111111111111111111111")
	assert.Contains(t, connectedWallets.ErcWallets, "0x2222222222222222222222222222222222222222")
	assert.NotContains(t, connectedWallets.ErcWallets, "0x3333333333333333333333333333333333333333")
	assert.NotContains(t, connectedWallets.SplWallets, "sol33333333333333333333333333333333333333333")
	assert.Contains(t, connectedWallets.SplWallets, "sol44444444444444444444444444444444444444444")
	assert.Contains(t, connectedWallets.SplWallets, "sol55555555555555555555555555555555555555555")
}

func TestGetUserConnectedWallets(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/connected_wallets")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.erc_wallets": `["0x1111111111111111111111111111111111111111","0x2222222222222222222222222222222222222222"]`,
		"data.spl_wallets": `["sol44444444444444444444444444444444444444444","sol55555555555555555555555555555555555555555"]`,
	})
}

func TestGetUserConnectedWalletsEmpty(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(4)+"/connected_wallets")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.spl_wallets": "[]",
		"data.erc_wallets": "[]",
	})
}
