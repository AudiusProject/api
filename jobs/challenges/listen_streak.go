package challenges

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ListenStreakProcessor implements challenge "e" — daily listen streak.
// Mirrors apps' listen_streak_endless_challenge.py.
//
// State lives in `challenge_listen_streak` (last_listen_date, listen_streak).
// Rules (per apps):
//   - First listen: streak = 1, last_listen_date = play timestamp.
//   - Subsequent listen >= 16h after last_listen_date: advance streak by 1.
//   - If >= 48h gap: reset streak to 1.
//   - Otherwise (<16h): ignore.
//
// user_challenges rows have specifier "<hex_user_id><YYYYMMDDHH>". When the
// streak crosses 7 days the row is completed; subsequent "endless" rows are
// minted at amount=1 each.
//
// NOTE: in challenges.json this challenge is currently active=false, so the
// processor is a no-op at present. Reconcile still does the right thing if
// it gets enabled.
type ListenStreakProcessor struct{}

func (p *ListenStreakProcessor) ChallengeID() string { return "e" }

const (
	listenStreakNextWindow   = 16 * time.Hour
	listenStreakBrokenWindow = 48 * time.Hour
	listenStreakTarget       = int32(7)
)

const listenStreakCheckpoint = "challenges:e:last_play_id"

// listenStreakPlay is one row from the plays scan.
type listenStreakPlay struct {
	id        int64
	userID    int64
	createdAt time.Time
}

func (p *ListenStreakProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	prev, err := readCheckpointInt(ctx, tx, listenStreakCheckpoint)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	// Pull new plays ordered by id. We deliberately use id ordering rather
	// than created_at because id is monotonic per insert and matches the
	// invariant apps' Python pipeline assumes (plays are processed in
	// arrival order).
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, created_at
		FROM plays
		WHERE id > $1 AND user_id IS NOT NULL
		ORDER BY id ASC
		LIMIT 50000
	`, prev)
	if err != nil {
		return fmt.Errorf("scan plays: %w", err)
	}
	var plays []listenStreakPlay
	for rows.Next() {
		var r listenStreakPlay
		if err := rows.Scan(&r.id, &r.userID, &r.createdAt); err != nil {
			rows.Close()
			return err
		}
		plays = append(plays, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(plays) == 0 {
		return nil
	}

	// Apply transitions in-process. We load existing streak state for
	// users with new plays in one go.
	userIDs := uniqueUserIDs(plays)
	type streakState struct {
		lastListen *time.Time
		streak     int32
	}
	state := make(map[int64]*streakState, len(userIDs))

	srows, err := tx.Query(ctx, `
		SELECT user_id, last_listen_date, listen_streak
		FROM challenge_listen_streak
		WHERE user_id = ANY($1)
	`, userIDs)
	if err != nil {
		return fmt.Errorf("load streak state: %w", err)
	}
	for srows.Next() {
		var uid int64
		var last *time.Time
		var streak int32
		if err := srows.Scan(&uid, &last, &streak); err != nil {
			srows.Close()
			return err
		}
		state[uid] = &streakState{lastListen: last, streak: streak}
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return err
	}

	// Walk plays in order, applying the transition rules per user. Each
	// play either advances or resets a streak — we batch the writes after
	// the simulation.
	type advance struct {
		newStreak  int32
		whenLogged time.Time
	}
	advancesByUser := make(map[int64][]advance)

	for _, pl := range plays {
		s, ok := state[pl.userID]
		if !ok {
			s = &streakState{lastListen: nil, streak: 0}
			state[pl.userID] = s
		}
		if s.lastListen == nil {
			s.streak = 1
			t := pl.createdAt
			s.lastListen = &t
			advancesByUser[pl.userID] = append(advancesByUser[pl.userID], advance{1, pl.createdAt})
			continue
		}
		gap := pl.createdAt.Sub(*s.lastListen)
		if gap < listenStreakNextWindow {
			continue // too soon, ignore
		}
		if gap >= listenStreakBrokenWindow {
			s.streak = 1
		} else {
			s.streak++
		}
		t := pl.createdAt
		s.lastListen = &t
		advancesByUser[pl.userID] = append(advancesByUser[pl.userID], advance{s.streak, pl.createdAt})
	}

	// Write updated streak state per user.
	for userID, s := range state {
		if _, err := tx.Exec(ctx, `
			INSERT INTO challenge_listen_streak (user_id, last_listen_date, listen_streak)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO UPDATE SET
				last_listen_date = EXCLUDED.last_listen_date,
				listen_streak = EXCLUDED.listen_streak
		`, userID, s.lastListen, s.streak); err != nil {
			return fmt.Errorf("upsert streak state: %w", err)
		}
	}

	// For each advance, mint or update the relevant user_challenge row.
	// Specifier format follows apps' new post-cutover format:
	//   first 7 days: "<hex>:YYYYMMDDHH" of *new streak boundary*
	//   endless     : same, one row per day after 7
	for userID, advances := range advancesByUser {
		for _, a := range advances {
			specifier := fmt.Sprintf("%x%s", userID, a.whenLogged.UTC().Format("2006010215"))
			// In the first-7-day window, current_step_count = streak (1..7),
			// is_complete when streak >= 7. After 7, this row is the
			// endless +1-per-day reward (step_count = 1, amount = 1).
			var stepCount int32 = listenStreakTarget
			report := a.newStreak
			if a.newStreak > listenStreakTarget {
				stepCount = 1
				report = 1
			}
			if err := UpsertUserChallenge(ctx, tx,
				p.ChallengeID(), specifier, userID, report, stepCount, amount,
			); err != nil {
				return fmt.Errorf("upsert listen-streak user_challenge: %w", err)
			}
		}
	}

	// Advance checkpoint to the last play id we processed.
	if err := writeCheckpointInt(ctx, tx, listenStreakCheckpoint, plays[len(plays)-1].id); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

func uniqueUserIDs(plays []listenStreakPlay) []int64 {
	seen := make(map[int64]struct{}, len(plays))
	out := make([]int64, 0, len(plays))
	for _, p := range plays {
		if _, ok := seen[p.userID]; ok {
			continue
		}
		seen[p.userID] = struct{}{}
		out = append(out, p.userID)
	}
	return out
}

// readCheckpointInt reads a named integer checkpoint from indexing_checkpoints,
// returning 0 if absent.
func readCheckpointInt(ctx context.Context, tx pgx.Tx, name string) (int64, error) {
	var v int64
	err := tx.QueryRow(ctx, "SELECT last_checkpoint FROM indexing_checkpoints WHERE tablename = $1", name).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func writeCheckpointInt(ctx context.Context, tx pgx.Tx, name string, value int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
		VALUES ($1, $2)
		ON CONFLICT (tablename) DO UPDATE SET last_checkpoint = EXCLUDED.last_checkpoint
	`, name, value)
	return err
}
