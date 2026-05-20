package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PlayCountMilestone implements challenges p1/p2/p3 — the 250/1k/10k play
// milestones from 2025 onward, restricted to verified artists.
// Mirrors apps/packages/discovery-provider/src/challenges/play_count_milestone_challenge_base.py.
//
// The play count is summed from aggregate_monthly_plays where timestamp >= 2025-01-01,
// joined to tracks the user owns. Each milestone has a "previous milestone"
// requirement: p2 requires p1 complete, p3 requires p2 complete (matches
// Python).
type PlayCountMilestone struct {
	ID                 string
	Threshold          int32 // step_count override (we still load from challenges row)
	PreviousMilestone  string
	SpecifierMilestone string // appended to specifier; matches Python's "hex:250"/"hex:1000"/"hex:10000"
}

// Per-milestone factory funcs. Each returns a Processor.
func NewPlayCount250Processor() Processor {
	return &PlayCountMilestone{ID: "p1", SpecifierMilestone: "250"}
}
func NewPlayCount1000Processor() Processor {
	return &PlayCountMilestone{ID: "p2", PreviousMilestone: "p1", SpecifierMilestone: "1000"}
}
func NewPlayCount10000Processor() Processor {
	return &PlayCountMilestone{ID: "p3", PreviousMilestone: "p2", SpecifierMilestone: "10000"}
}

func (p *PlayCountMilestone) ChallengeID() string { return p.ID }

func (p *PlayCountMilestone) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ID)
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active || c.StepCount == nil {
		return nil
	}
	stepCount := *c.StepCount
	amount := c.AmountInt()

	// Find verified artists with their total 2025+ play count. We use
	// aggregate_monthly_plays which is cheap to scan.
	//
	// The SQL only takes the (optional) previous-milestone id as a
	// parameter; stepCount is applied in Go after the scan.
	prevFilter := "true"
	var prevArgs []any
	if p.PreviousMilestone != "" {
		prevFilter = `EXISTS (
			SELECT 1 FROM user_challenges uc
			WHERE uc.challenge_id = $1
			  AND uc.user_id = u.user_id
			  AND uc.is_complete = true
		)`
		prevArgs = []any{p.PreviousMilestone}
	}

	query := fmt.Sprintf(`
		SELECT u.user_id, COALESCE(SUM(amp.count), 0)::int AS play_count
		FROM users u
		JOIN tracks t ON t.owner_id = u.user_id
		    AND t.is_current = true
		    AND t.is_delete = false
		JOIN aggregate_monthly_plays amp ON amp.play_item_id = t.track_id
		    AND amp.timestamp >= DATE '2025-01-01'
		WHERE u.is_current = true
		  AND u.is_verified = true
		  AND u.is_deactivated = false
		  AND %s
		GROUP BY u.user_id
		HAVING COALESCE(SUM(amp.count), 0) > 0
	`, prevFilter)

	rows, err := tx.Query(ctx, query, prevArgs...)
	if err != nil {
		return fmt.Errorf("scan play counts: %w", err)
	}
	type pcRow struct {
		userID    int64
		playCount int32
	}
	var results []pcRow
	for rows.Next() {
		var r pcRow
		if err := rows.Scan(&r.userID, &r.playCount); err != nil {
			rows.Close()
			return err
		}
		results = append(results, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range results {
		report := r.playCount
		if report > stepCount {
			report = stepCount
		}
		specifier := SpecifierFromUserID(r.userID) + ":" + p.SpecifierMilestone
		if err := UpsertUserChallenge(ctx, tx,
			p.ID, specifier, r.userID, report, stepCount, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
