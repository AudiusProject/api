package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FirstWeeklyCommentProcessor implements challenge "c" — each ISO-week,
// the user's first non-deleted, visible comment mints one user_challenge row.
//
// Specifier shape (matches apps):
//
//	<hex_user_id>:<YYYY><WW>   where WW is two-digit ISO week number
//
// Idempotency is by specifier — re-scanning the same week never adds
// duplicate rows. Each subsequent week the user posts, a new specifier
// (and thus a new row) is minted.
type FirstWeeklyCommentProcessor struct{}

func (p *FirstWeeklyCommentProcessor) ChallengeID() string { return "c" }

func (p *FirstWeeklyCommentProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	// One row per (user, isoyear, isoweek). EXTRACT(ISOYEAR / WEEK) gives
	// the ISO-8601 week numbers Python's date.isocalendar() returns.
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT user_id,
		       EXTRACT(ISOYEAR FROM created_at)::int AS iso_year,
		       EXTRACT(WEEK    FROM created_at)::int AS iso_week
		FROM comments
		WHERE is_delete = false
		  AND is_visible = true
	`)
	if err != nil {
		return fmt.Errorf("scan comments: %w", err)
	}
	type weekRow struct {
		userID  int64
		isoYear int
		isoWeek int
	}
	var results []weekRow
	for rows.Next() {
		var r weekRow
		if err := rows.Scan(&r.userID, &r.isoYear, &r.isoWeek); err != nil {
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
		specifier := fmt.Sprintf("%x:%d%02d", r.userID, r.isoYear, r.isoWeek)
		if err := UpsertUserChallenge(ctx, tx,
			p.ChallengeID(), specifier, r.userID, 1, 1, amount,
		); err != nil {
			return fmt.Errorf("upsert: %w", err)
		}
	}
	return nil
}
