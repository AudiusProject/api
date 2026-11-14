package api

import (
	"bytes"
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRetiredRewardClaimsRejectBeforeDependencies(t *testing.T) {
	server := &ApiServer{} // No database, key or Solana client may be needed.
	app := fiber.New()
	app.Post("/rewards/claim", server.v1ClaimRewards)
	for _, id := range []string{"e", "tp", "dvl", "rbig1", "unknown"} {
		req := httptest.NewRequest("POST", "/rewards/claim", bytes.NewBufferString(`{"userId":"L50xn","challengeId":"`+id+`"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 410, res.StatusCode)
		res.Body.Close()
	}
}

func TestProgrammableRewardRoutesRetired(t *testing.T) {
	app := testAppWithFixtures(t)
	for _, path := range []string{"/v1/rewards/code", "/v1/coins/mint/redeem", "/v1/coins/mint/redeem/code"} {
		status, _ := testPost(t, app, path, []byte(`{}`), nil)
		require.Equal(t, 410, status)
	}
}

func TestRetiredChallengeDoesNotGenerateNotification(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	function, err := os.ReadFile("../ddl/functions/handle_user_challenges.sql")
	require.NoError(t, err)
	_, err = app.pool.Exec(ctx, string(function))
	require.NoError(t, err)
	_, err = app.pool.Exec(ctx, `INSERT INTO user_challenges (challenge_id,user_id,specifier,is_complete,current_step_count,amount,completed_at,completed_blocknumber)
 VALUES ('e',402,'retirement-test',true,1,1,now(),200)`)
	require.NoError(t, err)
	var count int
	err = app.pool.QueryRow(ctx, "SELECT COUNT(*) FROM notification WHERE data->>'specifier' = 'retirement-test'").Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}
