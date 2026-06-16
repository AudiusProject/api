package api

import (
	"context"
	"testing"

	"api.audius.co/api/testdata"
	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostV1UsersPing(t *testing.T) {
	app := emptyTestApp(t)

	_, err := app.pool.Exec(context.Background(), `
		INSERT INTO public.blocks (blockhash, parenthash, number)
		VALUES ('block1', 'block0', 101)
		ON CONFLICT DO NOTHING;`)
	require.NoError(t, err)

	database.SeedTable(app.pool.Replicas[0], "users", testdata.UserFixtures)

	// user 1 = wallet 0x7d273271690538cf855e5b3002a0dd8c154bb060, encoded = 7eP5n
	wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"

	t.Run("authenticated request returns 200", func(t *testing.T) {
		status, body := testPostWithWallet(t, app, "/v1/users/me/ping?user_id=7eP5n", wallet, nil, nil)
		assert.Equal(t, 200, status, "body: %s", string(body))
	})

	t.Run("missing user_id returns 400", func(t *testing.T) {
		status, _ := testPostWithWallet(t, app, "/v1/users/me/ping", wallet, nil, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("unauthenticated request with user_id returns 403", func(t *testing.T) {
		status, _ := testPost(t, app, "/v1/users/me/ping?user_id=7eP5n", nil, nil)
		assert.Equal(t, 403, status)
	})
}
