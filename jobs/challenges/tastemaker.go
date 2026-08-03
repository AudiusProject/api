package challenges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TastemakerProcessor implements challenge "t" — for each currently-top
// trending track (week range), the earliest tastemakerThreshold reposters
// and earliest tastemakerThreshold savers earn the challenge.
// Mirrors apps' index_tastemaker.py.
//
// Per the user direction, tastemakerTrackLimit was bumped from apps'
// historical 5 to 10. The notification threshold (per-track number of
// reposters/savers eligible) stays at 10 to match apps.
//
// Specifier (matches apps): <hex_user_id>:t:<hex_track_id>
// (playlists would use ":p:" but trending_playlist tastemakers aren't
// in apps today, so we only handle tracks.)
type TastemakerProcessor struct{}

func (p *TastemakerProcessor) ChallengeID() string { return "t" }

const (
	tastemakerTrackLimit        = 10 // top-N trending tracks considered
	tastemakerThresholdPerTrack = 10 // earliest N reposters / savers per track
)

func (p *TastemakerProcessor) Reconcile(ctx context.Context, tx pgx.Tx) error {
	c, ok, err := LoadChallenge(ctx, tx, p.ChallengeID())
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok || !c.Active {
		return nil
	}
	amount := c.AmountInt()

	// Pick top-N tracks for the current week from the (already-running)
	// trending score job's output. We look at TRACKS type, pnagD version,
	// week range — matches what apps' top-trending lookup returns.
	topRows, err := tx.Query(ctx, `
		SELECT track_id
		FROM track_trending_scores
		WHERE type = 'TRACKS' AND version = 'pnagD' AND time_range = 'week'
		ORDER BY score DESC, track_id DESC
		LIMIT $1
	`, tastemakerTrackLimit)
	if err != nil {
		return fmt.Errorf("scan top trending: %w", err)
	}
	var trackIDs []int64
	for topRows.Next() {
		var tid int64
		if err := topRows.Scan(&tid); err != nil {
			topRows.Close()
			return err
		}
		trackIDs = append(trackIDs, tid)
	}
	topRows.Close()
	if err := topRows.Err(); err != nil {
		return err
	}

	if len(trackIDs) == 0 {
		return nil
	}

	// For each top track, find the union of (earliest 10 reposters) and
	// (earliest 10 savers). Each eligible user gets one row per track.
	for _, trackID := range trackIDs {
		userRows, err := tx.Query(ctx, `
			(
				SELECT user_id FROM reposts
				WHERE is_current = true AND is_delete = false
				  AND repost_type = 'track'
				  AND repost_item_id = $1
				ORDER BY created_at ASC
				LIMIT $2
			)
			UNION
			(
				SELECT user_id FROM saves
				WHERE is_current = true AND is_delete = false
				  AND save_type = 'track'
				  AND save_item_id = $1
				ORDER BY created_at ASC
				LIMIT $2
			)
		`, trackID, tastemakerThresholdPerTrack)
		if err != nil {
			return fmt.Errorf("scan tastemakers for track %d: %w", trackID, err)
		}
		var userIDs []int64
		for userRows.Next() {
			var uid int64
			if err := userRows.Scan(&uid); err != nil {
				userRows.Close()
				return err
			}
			userIDs = append(userIDs, uid)
		}
		userRows.Close()
		if err := userRows.Err(); err != nil {
			return err
		}

		for _, uid := range userIDs {
			specifier := fmt.Sprintf("%x:t:%x", uid, trackID)
			if err := UpsertUserChallenge(ctx, tx,
				p.ChallengeID(), specifier, uid, 1, 1, amount,
			); err != nil {
				return fmt.Errorf("upsert: %w", err)
			}
		}
	}
	return nil
}
