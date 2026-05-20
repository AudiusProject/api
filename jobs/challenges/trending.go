package challenges

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TrendingProcessor implements the three trending-reward challenges:
//
//	tt  trending_track
//	tut trending_underground_track
//	tp  trending_playlist
//
// Mirrors apps' trending_challenge.py + calculate_trending_challenges.py.
//
// Once per week (Friday UTC, only ever once per Friday) we snapshot the top
// 10 entries from track_trending_scores / playlist_trending_scores for the
// week time range and version, write a trending_results row per entry, and
// mint a user_challenges row for the entity owner. The per-rank reward
// amount mirrors apps:
//
//	track / underground: ranks 1-5 → 1000, ranks 6-10 → 100
//	playlist:            ranks 1-5 →  100, ranks 6-10 →  10
type TrendingProcessor struct {
	ID          string // "tt", "tut", or "tp"
	EntityKind  string // "track" or "playlist"
	TableName   string // "track_trending_scores" or "playlist_trending_scores"
	TrendingTyp string // value used in track_trending_scores.type column
	Version     string // strategy version, e.g. "pnagD"
}

func NewTrendingTrackProcessor() Processor {
	return &TrendingProcessor{ID: "tt", EntityKind: "track", TableName: "track_trending_scores", TrendingTyp: "TRACKS", Version: "pnagD"}
}
func NewTrendingUndergroundProcessor() Processor {
	return &TrendingProcessor{ID: "tut", EntityKind: "track", TableName: "track_trending_scores", TrendingTyp: "UNDERGROUND_TRACKS", Version: "pnagD"}
}
func NewTrendingPlaylistProcessor() Processor {
	return &TrendingProcessor{ID: "tp", EntityKind: "playlist", TableName: "playlist_trending_scores", TrendingTyp: "PLAYLISTS", Version: "pnagD"}
}

const trendingTopN = 10

func (p *TrendingProcessor) ChallengeID() string { return p.ID }

// amountForRank mirrors apps' TRENDING_TRACK_AMOUNTS_BY_RANK / playlist
// equivalent. Ranks 1-5 pay 10× ranks 6-10.
func (p *TrendingProcessor) amountForRank(rank int32) int32 {
	if p.EntityKind == "playlist" {
		if rank <= 5 {
			return 100
		}
		return 10
	}
	if rank <= 5 {
		return 1000
	}
	return 100
}

func (p *TrendingProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}

	now := time.Now().UTC()
	// Only run on Fridays — mirrors apps' get_is_valid_timestamp.
	// We use UTC rather than apps' America/Los_Angeles since the
	// container timezone shouldn't matter for fairness purposes; we
	// just need a stable weekly anchor.
	if now.Weekday() != time.Friday {
		return nil
	}
	// Stable date-of-this-Friday in UTC.
	weekDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Idempotent: if trending_results already has rows for this
	// (type, version, week), assume we've already paid out for this week.
	var alreadyPaid bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM trending_results
			WHERE type = $1 AND version = $2 AND week = $3
		)
	`, p.TrendingTyp, p.Version, weekDate).Scan(&alreadyPaid); err != nil {
		return fmt.Errorf("check trending_results: %w", err)
	}
	if alreadyPaid {
		return nil
	}

	// Pull the top-N entries with their owners. Both score tables share
	// the same shape minus the entity id column name.
	var rows pgx.Rows
	if p.EntityKind == "track" {
		rows, err = tx.Query(ctx, `
			SELECT s.track_id AS entity_id, t.owner_id AS user_id
			FROM track_trending_scores s
			JOIN tracks t ON t.track_id = s.track_id AND t.is_current = true
			WHERE s.type = $1 AND s.version = $2 AND s.time_range = 'week'
			ORDER BY s.score DESC NULLS LAST
			LIMIT $3
		`, p.TrendingTyp, p.Version, trendingTopN)
	} else {
		rows, err = tx.Query(ctx, `
			SELECT s.playlist_id AS entity_id, pl.playlist_owner_id AS user_id
			FROM playlist_trending_scores s
			JOIN playlists pl ON pl.playlist_id = s.playlist_id AND pl.is_current = true
			WHERE s.type = $1 AND s.version = $2 AND s.time_range = 'week'
			ORDER BY s.score DESC NULLS LAST
			LIMIT $3
		`, p.TrendingTyp, p.Version, trendingTopN)
	}
	if err != nil {
		return fmt.Errorf("scan trending scores: %w", err)
	}
	type entry struct {
		entityID int64
		userID   int64
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.entityID, &e.userID); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	for idx, e := range entries {
		rank := int32(idx + 1)
		rewardAmount := p.amountForRank(rank)
		entityIDStr := fmt.Sprintf("%d", e.entityID)
		if _, err := tx.Exec(ctx, `
			INSERT INTO trending_results (user_id, id, rank, type, version, week)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING
		`, e.userID, entityIDStr, rank, p.TrendingTyp, p.Version, weekDate); err != nil {
			return fmt.Errorf("insert trending_results: %w", err)
		}
		// Specifier matches apps: "{week}:{rank}".
		specifier := fmt.Sprintf("%s:%d", weekDate.Format("2006-01-02"), rank)
		if err := UpsertUserChallenge(ctx, tx,
			p.ID, specifier, e.userID, 1, 1, rewardAmount,
		); err != nil {
			return fmt.Errorf("upsert trending user_challenge: %w", err)
		}
	}
	return nil
}

// Compile-time check the helper is referenced — avoids "declared and not used"
// when this file is the only consumer of pgx.ErrNoRows.
var _ = errors.Is
