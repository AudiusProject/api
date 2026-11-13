package api

import (
	"api.audius.co/api/dbv1"
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTrendingOnlyUndisbursed(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	_, err := app.pool.Exec(ctx, `
 INSERT INTO challenges (id,type,amount,active,step_count,cooldown_days) VALUES
 ('tt','trending','1000',true,1,0), ('tut','trending','1000',true,1,0)
 ON CONFLICT (id) DO UPDATE SET active = true;
 INSERT INTO user_challenges (challenge_id,user_id,specifier,is_complete,current_step_count,amount,completed_blocknumber,completed_at)
 VALUES ('tt',402,'2026-11-13:1',true,1,1000,200,now()), ('tut',402,'2026-11-13:6',true,1,100,200,now());
 `)
	require.NoError(t, err)
	// Omitted IDs must not let a claim-all request pay old challenge fixtures.
	rows, err := app.queries.GetUndisbursedChallenges(ctx, dbv1.GetUndisbursedChallengesParams{UserID: 402})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.True(t, isTrendingReward(row.ChallengeID))
	}
	// Explicit retired IDs are excluded at the SQL boundary as well.
	rows, err = app.queries.GetUndisbursedChallenges(ctx, dbv1.GetUndisbursedChallengesParams{UserID: 402, ChallengeID: "e"})
	require.NoError(t, err)
	require.Empty(t, rows)
	status, body := testGet(t, app, "/v1/challenges/undisbursed?challenge_id=tt&completed_blocknumber=199")
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": float64(1), "data.0.specifier": "2026-11-13:1"})
	status, body = testGet(t, app, "/v1/challenges/undisbursed?challenge_id=e")
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": float64(0)})
	// Active flags and payments must apply to both listing and actual claiming.
	_, err = app.pool.Exec(ctx, `UPDATE challenges SET active = false WHERE id = 'tt';
 INSERT INTO sol_reward_disbursements (signature,instruction_index,amount,slot,user_bank,challenge_id,specifier,recipient_eth_address)
 VALUES ('paid-tut',0,10000000000,1,'test-bank','tut','2026-11-13:6','0x0000000000000000000000000000000000000001');`)
	require.NoError(t, err)
	rows, err = app.queries.GetUndisbursedChallenges(ctx, dbv1.GetUndisbursedChallengesParams{UserID: 402})
	require.NoError(t, err)
	require.Empty(t, rows)
	status, body = testGet(t, app, "/v1/challenges/undisbursed")
	require.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": float64(0)})
}
