package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

// rawPlay is one row from the plays table fed into the listening-history merge.
type rawPlay struct {
	ID        int64
	UserID    int64
	TrackID   int64
	CreatedAt time.Time
}

// UserListeningHistoryJob materializes per-user listening history from the
// plays table into user_listening_history.listening_history JSONB.
// Mirrors apps/packages/discovery-provider/src/tasks/user_listening_history/.
//
// On each run:
//   - reads the last processed plays.id checkpoint from indexing_checkpoints,
//   - selects the next UserListeningHistoryBatchSize plays with non-null user_id,
//   - merges each user's new plays into their existing history (per-track
//     latest timestamp + cumulative play_count, sorted desc by timestamp,
//     capped at UserListeningHistoryTrackLimit entries),
//   - rewrites the listening_history JSONB blob for each touched user,
//   - advances the checkpoint to the highest play.id processed.
type UserListeningHistoryJob struct {
	pool   database.DbPool
	logger *zap.Logger

	batchSize int
	limit     int

	mutex     sync.Mutex
	isRunning bool
}

const (
	// UserListeningHistoryCheckpoint is the indexing_checkpoints.tablename
	// used to track the highest plays.id already merged.
	UserListeningHistoryCheckpoint = "user_listening_history"
	// UserListeningHistoryBatchSize caps how many plays are merged per run.
	UserListeningHistoryBatchSize = 100_000
	// UserListeningHistoryTrackLimit caps how many tracks each user's
	// listening_history JSONB retains.
	UserListeningHistoryTrackLimit = 1000
)

func NewUserListeningHistoryJob(cfg config.Config, pool database.DbPool) *UserListeningHistoryJob {
	return &UserListeningHistoryJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("UserListeningHistoryJob"),
		batchSize: UserListeningHistoryBatchSize,
		limit:     UserListeningHistoryTrackLimit,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *UserListeningHistoryJob) ScheduleEvery(ctx context.Context, interval time.Duration) *UserListeningHistoryJob {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once.
func (j *UserListeningHistoryJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// listenEntry mirrors apps' ListenHistory.to_dict() shape so the persisted
// JSONB stays compatible with reads in apps and api/.
type listenEntry struct {
	TrackID   int64  `json:"track_id"`
	Timestamp string `json:"timestamp"`
	PlayCount int64  `json:"play_count"`
}

func (j *UserListeningHistoryJob) run(ctx context.Context) error {
	start := time.Now()
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	prev, err := getCheckpoint(ctx, j.pool, UserListeningHistoryCheckpoint)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}

	rows, err := j.pool.Query(ctx, `
		SELECT id, user_id, play_item_id, created_at
		FROM plays
		WHERE id > $1 AND user_id IS NOT NULL
		ORDER BY id ASC
		LIMIT $2
	`, prev, j.batchSize)
	if err != nil {
		return fmt.Errorf("read plays: %w", err)
	}

	var newPlays []rawPlay
	for rows.Next() {
		var p rawPlay
		if err := rows.Scan(&p.ID, &p.UserID, &p.TrackID, &p.CreatedAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan play: %w", err)
		}
		newPlays = append(newPlays, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iter plays: %w", err)
	}

	if len(newPlays) == 0 {
		j.logger.Debug("No new plays for user listening history",
			zap.Duration("duration", time.Since(start)))
		return nil
	}

	// Group new plays by user.
	byUser := make(map[int64][]rawPlay)
	for _, p := range newPlays {
		byUser[p.UserID] = append(byUser[p.UserID], p)
	}

	userIDs := make([]int64, 0, len(byUser))
	for uid := range byUser {
		userIDs = append(userIDs, uid)
	}

	// Load existing histories for any users with new plays in one query.
	existing := make(map[int64][]listenEntry, len(userIDs))
	existingRows, err := j.pool.Query(ctx,
		"SELECT user_id, listening_history FROM user_listening_history WHERE user_id = ANY($1)",
		userIDs)
	if err != nil {
		return fmt.Errorf("read existing history: %w", err)
	}
	for existingRows.Next() {
		var uid int64
		var raw []byte
		if err := existingRows.Scan(&uid, &raw); err != nil {
			existingRows.Close()
			return fmt.Errorf("scan history: %w", err)
		}
		var entries []listenEntry
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &entries); err != nil {
				j.logger.Warn("Could not parse existing listening_history; treating as empty",
					zap.Int64("user_id", uid), zap.Error(err))
				entries = nil
			}
		}
		existing[uid] = entries
	}
	existingRows.Close()
	if err := existingRows.Err(); err != nil {
		return fmt.Errorf("iter history: %w", err)
	}

	maxID := newPlays[len(newPlays)-1].ID

	// Merge per user and write back. We do per-user UPSERTs rather than a
	// batched COPY because the merge is non-trivial (per-track latest
	// timestamp + cumulative play_count) and a single batched write would
	// require recomputing every user's merged blob in SQL.
	updated := 0
	for uid, plays := range byUser {
		merged := mergeListeningHistory(existing[uid], plays, j.limit)
		blob, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("marshal merged history: %w", err)
		}
		_, err = j.pool.Exec(ctx, `
			INSERT INTO user_listening_history (user_id, listening_history)
			VALUES ($1, $2::jsonb)
			ON CONFLICT (user_id) DO UPDATE SET listening_history = EXCLUDED.listening_history
		`, uid, blob)
		if err != nil {
			return fmt.Errorf("upsert history for user %d: %w", uid, err)
		}
		updated++
	}

	if err := saveCheckpoint(ctx, j.pool, UserListeningHistoryCheckpoint, maxID); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	j.logger.Info("Indexed user listening history",
		zap.Int("plays_processed", len(newPlays)),
		zap.Int("users_touched", updated),
		zap.Int64("new_checkpoint", maxID),
		zap.Duration("duration", time.Since(start)))
	return nil
}

// mergeListeningHistory combines an existing history with new plays for the
// same user. For each track, the entry keeps the latest timestamp and the
// cumulative play_count (existing.play_count + count of new plays for that
// track). The result is sorted by timestamp desc and capped at `limit`.
//
// Mirrors apps' separate_new_plays + sort_listening_history_desc_by_timestamp
// behavior. Exported for tests.
func mergeListeningHistory(existing []listenEntry, newPlays []rawPlay, limit int) []listenEntry {
	type acc struct {
		latest    time.Time
		latestStr string
		count     int64
	}
	byTrack := make(map[int64]*acc)

	// Seed with existing entries.
	for _, e := range existing {
		t, _ := parseHistoryTimestamp(e.Timestamp)
		byTrack[e.TrackID] = &acc{
			latest:    t,
			latestStr: e.Timestamp,
			count:     e.PlayCount,
		}
	}

	// Apply new plays.
	for _, p := range newPlays {
		a, ok := byTrack[p.TrackID]
		if !ok {
			a = &acc{}
			byTrack[p.TrackID] = a
		}
		if p.CreatedAt.After(a.latest) {
			a.latest = p.CreatedAt
			a.latestStr = formatHistoryTimestamp(p.CreatedAt)
		}
		a.count++
	}

	out := make([]listenEntry, 0, len(byTrack))
	for trackID, a := range byTrack {
		out = append(out, listenEntry{
			TrackID:   trackID,
			Timestamp: a.latestStr,
			PlayCount: a.count,
		})
	}
	sort.SliceStable(out, func(i, k int) bool {
		// Latest first; ties broken by track_id desc to keep deterministic order.
		ti, _ := parseHistoryTimestamp(out[i].Timestamp)
		tk, _ := parseHistoryTimestamp(out[k].Timestamp)
		if !ti.Equal(tk) {
			return ti.After(tk)
		}
		return out[i].TrackID > out[k].TrackID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// formatHistoryTimestamp matches Python's `str(datetime)` shape closely
// enough to be parseable by existing consumers. Python uses
// "YYYY-MM-DD HH:MM:SS[.ffffff][+TZ]" — Go's "2006-01-02 15:04:05.999999"
// is the equivalent for timezone-naive timestamps stored in `plays`.
func formatHistoryTimestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05.999999")
}

// parseHistoryTimestamp accepts both the format we write and a few
// permissive variants apps might have written (with trailing TZ).
func parseHistoryTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	// Fall back to zero time on bad input so the entry sorts last.
	return time.Time{}, errors.New("unrecognized timestamp")
}
