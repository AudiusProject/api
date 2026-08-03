package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PlayCountMilestonesProcessor implements challenges p1/p2/p3 — the 250/1k/10k
// play milestones from 2025 onward, restricted to verified artists.
// Mirrors apps/packages/discovery-provider/src/challenges/play_count_milestone_challenge_base.py.
//
// All three tiers derive from the same number: a verified artist's total 2025+
// play count, summed from aggregate_monthly_plays over the tracks they own. A
// tier is eligible once the count reaches the previous tier's threshold (p2
// needs >=250, p3 needs >=1000), matching Python's "previous milestone
// complete" gating — since a milestone completes exactly when the count crosses
// its step_count, gating on the live count is equivalent and doesn't need a
// second tick for the p1->p2->p3 cascade to propagate.
//
// Incremental: the old design ran one non-incremental GROUP BY per tier every
// tick — a Parallel Seq Scan of ~485k tracks producing ~1,867 rows in ~49s,
// times three. We instead checkpoint plays.id and recompute only the verified
// artists whose tracks were played since the checkpoint, in a single pass for
// all three tiers. See incremental.go.
type PlayCountMilestonesProcessor struct{}

func NewPlayCountMilestonesProcessor() Processor { return &PlayCountMilestonesProcessor{} }

// ChallengeID returns p1 as a representative id; it is only used for error
// logging by the umbrella job. Reconcile handles all three tiers.
func (p *PlayCountMilestonesProcessor) ChallengeID() string { return "p1" }

const playCountMilestonesCheckpoint = "challenges:play_count_milestones:last_play_id"

// playCountTiers are the milestone challenges in ascending order. specifier is
// appended to the per-user specifier and matches Python's "hex:250" form.
var playCountTiers = []struct {
	id        string
	specifier string
}{
	{id: "p1", specifier: "250"},
	{id: "p2", specifier: "1000"},
	{id: "p3", specifier: "10000"},
}

type loadedTier struct {
	id        string
	specifier string
	stepCount int32
	amount    int32
}

// playCountMilestonesDirtySQL surfaces verified artists whose tracks were played
// since the checkpoint. plays is append-only with a monotonic serial id, so
// plays.id is the high-water mark (aggregate_monthly_plays is updated in place
// and carries no monotonic column). The join filters to verified, non-deactivated
// artists so we only ever recompute users who can actually earn the reward.
const playCountMilestonesDirtySQL = `
	SELECT t.owner_id, p.id
	FROM plays p
	JOIN tracks t ON t.track_id = p.play_item_id
		AND t.is_current = true
		AND t.is_delete = false
	JOIN users u ON u.user_id = t.owner_id
		AND u.is_current = true
		AND u.is_verified = true
		AND u.is_deactivated = false
	WHERE p.id > $1
	ORDER BY p.id ASC
	LIMIT $2
`

func (p *PlayCountMilestonesProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	tiers, err := p.loadTiers(ctx, tx)
	if err != nil {
		return err
	}
	if len(tiers) == 0 {
		return nil
	}
	return reconcileIncrementalUsers(ctx, tx, playCountMilestonesCheckpoint, playCountMilestonesDirtySQL,
		func(ctx context.Context, tx pgx.Tx, userIDs []int64) error {
			return p.recompute(ctx, tx, userIDs, tiers)
		})
}

// loadTiers loads the active milestone challenges from the challenges table, in
// ascending order. Each tier's threshold and amount come from its row.
func (p *PlayCountMilestonesProcessor) loadTiers(ctx context.Context, tx pgx.Tx) ([]loadedTier, error) {
	var out []loadedTier
	for _, t := range playCountTiers {
		c, ok, err := LoadChallenge(ctx, tx, t.id)
		if err != nil {
			return nil, fmt.Errorf("load challenge %s: %w", t.id, err)
		}
		if !ok || !c.Active || c.StepCount == nil {
			continue
		}
		out = append(out, loadedTier{
			id:        t.id,
			specifier: t.specifier,
			stepCount: *c.StepCount,
			amount:    c.AmountInt(),
		})
	}
	return out, nil
}

func (p *PlayCountMilestonesProcessor) recompute(ctx context.Context, tx pgx.Tx, userIDs []int64, tiers []loadedTier) error {
	// Total 2025+ play count for just the affected verified artists. Bounded by
	// the dirty set (a handful of users per tick), this is a cheap indexed
	// aggregation rather than the old full-table GROUP BY.
	rows, err := tx.Query(ctx, `
		SELECT t.owner_id, COALESCE(SUM(amp.count), 0)::int AS play_count
		FROM tracks t
		JOIN aggregate_monthly_plays amp ON amp.play_item_id = t.track_id
			AND amp.timestamp >= DATE '2025-01-01'
		WHERE t.owner_id = ANY($1)
			AND t.is_current = true
			AND t.is_delete = false
		GROUP BY t.owner_id
	`, userIDs)
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
		if r.playCount <= 0 {
			continue
		}
		// Walk tiers ascending; each tier is eligible only once the count
		// reaches the previous tier's threshold (the p1->p2->p3 gate).
		prevThreshold := int32(0)
		for _, tier := range tiers {
			if r.playCount < prevThreshold {
				break
			}
			report := r.playCount
			if report > tier.stepCount {
				report = tier.stepCount
			}
			specifier := SpecifierFromUserID(r.userID) + ":" + tier.specifier
			if err := UpsertUserChallenge(ctx, tx,
				tier.id, specifier, r.userID, report, tier.stepCount, tier.amount,
			); err != nil {
				return fmt.Errorf("upsert %s: %w", tier.id, err)
			}
			prevThreshold = tier.stepCount
		}
	}
	return nil
}
