package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RemixContestWinnerProcessor implements challenge "w" — winners of a
// remix contest hosted by a verified user. Mirrors apps' dispatch in
// entity_manager/entities/event.py:218 plus the gates in
// challenges/remix_contest_winner_challenge.py.
//
// Source: events table where event_type='remix_contest', is_deleted=false,
// and event_data->>'winners' is a JSON array of *track IDs*. For each
// winner track, we mint a row for the *remixer* (track owner) — same as
// apps' apps' dispatcher.
//
// Specifier (matches apps): <hex_contest_event_id>:<hex_remixer_user_id>
//
// Caps (per apps):
//   - host must be verified
//   - max 5 winners per contest (apps MAX_WINNERS_PER_CONTEST)
//   - one row per (remixer, contest) — enforced by specifier uniqueness
//   - max 5 winner rewards per host per rolling week
//     (apps MAX_WINNER_REWARDS_PER_HOST_PER_WEEK)
type RemixContestWinnerProcessor struct{}

func (p *RemixContestWinnerProcessor) ChallengeID() string { return "w" }

const (
	remixContestMaxWinnersPerContest    = 5
	remixContestMaxWinnerRewardsPerWeek = 5
)

func (p *RemixContestWinnerProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	// Pull all remix contests whose winners array is non-empty, joined to
	// the host (must be verified). We unnest the JSON array of winner
	// track ids and resolve each one to a remixer via tracks.owner_id.
	// jsonb_array_elements_text yields each array element as text, which
	// casts cleanly to int regardless of whether the JSON had `[123,456]`
	// (raw ints) or `["123","456"]` (string ints) — both are the shapes
	// apps' entity_manager has historically written.
	rows, err := tx.Query(ctx, `
		SELECT e.event_id, e.user_id AS host_user_id, t.owner_id AS remixer_user_id
		FROM events e
		JOIN users u ON u.user_id = e.user_id
		     AND u.is_current = true AND u.is_verified = true
		CROSS JOIN LATERAL jsonb_array_elements_text(
		    COALESCE(e.event_data->'winners', '[]'::jsonb)
		) AS w(track_id_str)
		JOIN tracks t ON t.track_id = w.track_id_str::int
		     AND t.is_current = true AND t.is_delete = false
		WHERE e.event_type = 'remix_contest'
		  AND e.is_deleted = false
	`)
	if err != nil {
		return fmt.Errorf("scan winner candidates: %w", err)
	}
	type winnerRow struct {
		contestID     int64
		hostUserID    int64
		remixerUserID int64
	}
	var results []winnerRow
	for rows.Next() {
		var r winnerRow
		if err := rows.Scan(&r.contestID, &r.hostUserID, &r.remixerUserID); err != nil {
			rows.Close()
			return err
		}
		results = append(results, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Per-contest winners-count cache to enforce the 5-per-contest cap.
	winnersByContest := make(map[int64]int)
	// Per-host weekly count cache, populated lazily.
	rewardsThisWeekByHost := make(map[int64]int)

	for _, r := range results {
		specifier := fmt.Sprintf("%x:%x", r.contestID, r.remixerUserID)

		// Skip already-minted (idempotent).
		var alreadyHave bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM user_challenges
			               WHERE challenge_id = 'w' AND specifier = $1)
		`, specifier).Scan(&alreadyHave); err != nil {
			return err
		}
		if alreadyHave {
			continue
		}

		// Per-contest cap (5 winners max).
		if _, ok := winnersByContest[r.contestID]; !ok {
			var n int
			contestPrefix := fmt.Sprintf("%x:", r.contestID)
			if err := tx.QueryRow(ctx, `
				SELECT COUNT(*) FROM user_challenges
				WHERE challenge_id = 'w' AND specifier LIKE $1
			`, contestPrefix+"%").Scan(&n); err != nil {
				return err
			}
			winnersByContest[r.contestID] = n
		}
		if winnersByContest[r.contestID] >= remixContestMaxWinnersPerContest {
			continue
		}

		// Per-host weekly cap (5 winner rewards per week).
		if _, ok := rewardsThisWeekByHost[r.hostUserID]; !ok {
			var n int
			if err := tx.QueryRow(ctx, `
				SELECT COUNT(*) FROM user_challenges uc
				JOIN events e ON e.event_id = SPLIT_PART(uc.specifier, ':', 1)::bigint
				WHERE uc.challenge_id = 'w'
				  AND e.user_id = $1
				  AND uc.created_at >= now() - INTERVAL '7 days'
			`, r.hostUserID).Scan(&n); err != nil {
				return err
			}
			rewardsThisWeekByHost[r.hostUserID] = n
		}
		if rewardsThisWeekByHost[r.hostUserID] >= remixContestMaxWinnerRewardsPerWeek {
			continue
		}

		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.remixerUserID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
		winnersByContest[r.contestID]++
		rewardsThisWeekByHost[r.hostUserID]++
	}
	return nil
}
