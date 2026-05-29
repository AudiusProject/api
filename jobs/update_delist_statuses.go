package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// UpdateDelistStatusesJob polls a trusted-notifier endpoint for delist
// status updates and applies them locally. Mirrors apps'
// src/tasks/update_delist_statuses.py.
//
// For each entity type (users, tracks) the job:
//  1. Reads the per-host/entity cursor from delist_status_cursor.
//  2. GETs <host>statuses/<entity>?batchSize=...&cursor=... with a delegate
//     signed Authorization header.
//  3. Inserts the raw status rows into user_delist_statuses /
//     track_delist_statuses (audit log).
//  4. Applies the most-recent status per id to users.is_available /
//     tracks.is_available + is_delete / is_deactivated, holding back any
//     status whose target row hasn't been indexed yet (matches apps'
//     wait-for-indexing logic).
//  5. Advances the cursor to the last `createdAt` from the batch.
//
// TrustedNotifierEndpoint is currently hardcoded to notifier.audius.co per
// the agreed scoping; promote to a config knob if/when multi-notifier
// support is needed.
type UpdateDelistStatusesJob struct {
	pool       database.DbPool
	logger     *zap.Logger
	endpoint   string
	delegateKey string
	httpClient *http.Client

	mutex     sync.Mutex
	isRunning bool
}

const (
	// TrustedNotifierEndpoint is the canonical trusted notifier base URL.
	// apps reads this from config; we hardcode it for now and revisit if
	// multi-notifier support is needed.
	TrustedNotifierEndpoint = "https://notifier.audius.co/"
	// delistBatchSize matches apps' DELIST_BATCH_SIZE.
	delistBatchSize = 5000
)

func NewUpdateDelistStatusesJob(cfg config.Config, pool database.DbPool) *UpdateDelistStatusesJob {
	return &UpdateDelistStatusesJob{
		pool:        pool,
		logger:      logging.NewZapLogger(cfg).Named("UpdateDelistStatusesJob"),
		endpoint:    TrustedNotifierEndpoint,
		delegateKey: cfg.DelegatePrivateKey,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *UpdateDelistStatusesJob) ScheduleEvery(ctx context.Context, interval time.Duration) *UpdateDelistStatusesJob {
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
func (j *UpdateDelistStatusesJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *UpdateDelistStatusesJob) run(ctx context.Context) error {
	if j.delegateKey == "" {
		j.logger.Debug("Skipping: delegate private key not configured")
		return nil
	}

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

	// apps uses the latest indexed block timestamp as the "current block
	// timestamp" to decide whether to wait for missing entities. Without
	// that hook here we just use now() — equivalent behavior when the
	// indexer is caught up, and slightly more eager when it isn't (we
	// skip waiting for entities whose delist was created before now,
	// which is the safe default).
	currentBlockTime := time.Now()

	for _, entity := range []string{"users", "tracks"} {
		if err := j.processEntity(ctx, entity, currentBlockTime); err != nil {
			j.logger.Error("processing delist entity failed",
				zap.String("entity", entity), zap.Error(err))
		}
	}
	return nil
}

type delistUserResp struct {
	Result struct {
		Users []delistUser `json:"users"`
	} `json:"result"`
}

type delistTrackResp struct {
	Result struct {
		Tracks []delistTrack `json:"tracks"`
	} `json:"result"`
}

type delistUser struct {
	CreatedAt string `json:"createdAt"`
	UserID    int64  `json:"userId"`
	Delisted  bool   `json:"delisted"`
	Reason    string `json:"reason"`
}

type delistTrack struct {
	CreatedAt string `json:"createdAt"`
	TrackID   int64  `json:"trackId"`
	OwnerID   int64  `json:"ownerId"`
	TrackCID  string `json:"trackCid"`
	Delisted  bool   `json:"delisted"`
	Reason    string `json:"reason"`
}

func (j *UpdateDelistStatusesJob) processEntity(ctx context.Context, entity string, currentBlockTime time.Time) error {
	cursorBefore, err := j.readCursor(ctx, entity)
	if err != nil {
		return fmt.Errorf("read cursor: %w", err)
	}

	pollURL := fmt.Sprintf("%sstatuses/%s?batchSize=%d", j.endpoint, entity, delistBatchSize)
	if !cursorBefore.IsZero() {
		// apps formats the cursor as RFC3339-ish "%Y-%m-%dT%H:%M:%S.%fZ" then
		// URL-escapes it. time.RFC3339Nano includes the fractional seconds
		// we need; queryEscape gives the same %-encoding apps applies.
		pollURL += "&cursor=" + url.QueryEscape(cursorBefore.UTC().Format("2006-01-02T15:04:05.999999Z"))
	}

	resp, err := signedHTTPGet(j.httpClient, pollURL, j.delegateKey)
	if err != nil {
		return fmt.Errorf("signed GET: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// drainResponse closes the body and constructs an error.
		return fmt.Errorf("notifier returned non-2xx: %w", drainResponse(resp))
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if entity == "users" {
		return j.processUsersBatch(ctx, body, currentBlockTime)
	}
	return j.processTracksBatch(ctx, body, currentBlockTime)
}

func (j *UpdateDelistStatusesJob) processUsersBatch(ctx context.Context, body []byte, currentBlockTime time.Time) error {
	var parsed delistUserResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parse users response: %w", err)
	}
	users := parsed.Result.Users
	if len(users) == 0 {
		return nil
	}

	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert raw audit-log rows.
	if err := insertUserDelistStatuses(ctx, tx, users); err != nil {
		return fmt.Errorf("insert user_delist_statuses: %w", err)
	}

	// Build per-user-latest-status map (input is asc by createdAt so later
	// wins).
	type entry struct {
		delisted  bool
		createdAt time.Time
	}
	latest := make(map[int64]entry, len(users))
	for _, u := range users {
		ts, err := parseDelistTimestamp(u.CreatedAt)
		if err != nil {
			j.logger.Warn("skipping user with unparseable createdAt",
				zap.Int64("user_id", u.UserID), zap.String("createdAt", u.CreatedAt))
			continue
		}
		latest[u.UserID] = entry{delisted: u.Delisted, createdAt: ts}
	}

	userIDs := make([]int64, 0, len(latest))
	for id := range latest {
		userIDs = append(userIDs, id)
	}
	existing, err := selectExistingIDs(ctx, tx, "SELECT user_id FROM users WHERE user_id = ANY($1) AND is_current = true", userIDs)
	if err != nil {
		return fmt.Errorf("select existing users: %w", err)
	}

	// Wait-for-indexing: if any missing user's delist timestamp is after
	// the current block timestamp, halt this batch — apps' invariant.
	for id, e := range latest {
		if _, ok := existing[id]; ok {
			continue
		}
		if e.createdAt.After(currentBlockTime) {
			j.logger.Info("waiting for unindexed user; skipping batch",
				zap.Int64("user_id", id))
			return nil
		}
	}

	for id := range existing {
		e := latest[id]
		var sql string
		if e.delisted {
			sql = `UPDATE users SET is_available = false, is_deactivated = true
			       WHERE user_id = $1 AND is_current = true AND is_available = true`
		} else {
			sql = `UPDATE users SET is_available = true, is_deactivated = false
			       WHERE user_id = $1 AND is_current = true AND is_available = false`
		}
		if _, err := tx.Exec(ctx, sql, id); err != nil {
			return fmt.Errorf("update user %d availability: %w", id, err)
		}
	}

	// Advance cursor to the last createdAt in the batch.
	lastCreatedAt, err := parseDelistTimestamp(users[len(users)-1].CreatedAt)
	if err != nil {
		return fmt.Errorf("parse final cursor: %w", err)
	}
	if err := upsertDelistCursor(ctx, tx, j.endpoint, "USERS", lastCreatedAt); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	j.logger.Info("Applied user delist batch",
		zap.Int("batch_size", len(users)),
		zap.Int("updated", len(existing)))
	return nil
}

func (j *UpdateDelistStatusesJob) processTracksBatch(ctx context.Context, body []byte, currentBlockTime time.Time) error {
	var parsed delistTrackResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parse tracks response: %w", err)
	}
	tracks := parsed.Result.Tracks
	if len(tracks) == 0 {
		return nil
	}

	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := insertTrackDelistStatuses(ctx, tx, tracks); err != nil {
		return fmt.Errorf("insert track_delist_statuses: %w", err)
	}

	type entry struct {
		delisted  bool
		createdAt time.Time
	}
	latest := make(map[int64]entry, len(tracks))
	for _, t := range tracks {
		ts, err := parseDelistTimestamp(t.CreatedAt)
		if err != nil {
			j.logger.Warn("skipping track with unparseable createdAt",
				zap.Int64("track_id", t.TrackID), zap.String("createdAt", t.CreatedAt))
			continue
		}
		latest[t.TrackID] = entry{delisted: t.Delisted, createdAt: ts}
	}

	trackIDs := make([]int64, 0, len(latest))
	for id := range latest {
		trackIDs = append(trackIDs, id)
	}
	existing, err := selectExistingIDs(ctx, tx, "SELECT track_id FROM tracks WHERE track_id = ANY($1) AND is_current = true", trackIDs)
	if err != nil {
		return fmt.Errorf("select existing tracks: %w", err)
	}

	for id, e := range latest {
		if _, ok := existing[id]; ok {
			continue
		}
		if e.createdAt.After(currentBlockTime) {
			j.logger.Info("waiting for unindexed track; skipping batch",
				zap.Int64("track_id", id))
			return nil
		}
	}

	for id := range existing {
		e := latest[id]
		var sql string
		if e.delisted {
			sql = `UPDATE tracks SET is_available = false, is_delete = true
			       WHERE track_id = $1 AND is_current = true AND is_available = true`
		} else {
			sql = `UPDATE tracks SET is_available = true, is_delete = false
			       WHERE track_id = $1 AND is_current = true AND is_available = false`
		}
		if _, err := tx.Exec(ctx, sql, id); err != nil {
			return fmt.Errorf("update track %d availability: %w", id, err)
		}
	}

	lastCreatedAt, err := parseDelistTimestamp(tracks[len(tracks)-1].CreatedAt)
	if err != nil {
		return fmt.Errorf("parse final cursor: %w", err)
	}
	if err := upsertDelistCursor(ctx, tx, j.endpoint, "TRACKS", lastCreatedAt); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	j.logger.Info("Applied track delist batch",
		zap.Int("batch_size", len(tracks)),
		zap.Int("updated", len(existing)))
	return nil
}

func (j *UpdateDelistStatusesJob) readCursor(ctx context.Context, entity string) (time.Time, error) {
	entityEnum := strings.ToUpper(entity)
	var t time.Time
	err := j.pool.QueryRow(ctx,
		"SELECT created_at FROM delist_status_cursor WHERE host = $1 AND entity = $2",
		j.endpoint, entityEnum,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

func upsertDelistCursor(ctx context.Context, tx pgx.Tx, host, entity string, createdAt time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO delist_status_cursor (host, entity, created_at)
		VALUES ($1, $2::delist_entity, $3)
		ON CONFLICT (host, entity) DO UPDATE SET created_at = EXCLUDED.created_at
	`, host, entity, createdAt)
	return err
}

func insertUserDelistStatuses(ctx context.Context, tx pgx.Tx, users []delistUser) error {
	if len(users) == 0 {
		return nil
	}
	createdAts := make([]string, len(users))
	userIDs := make([]int64, len(users))
	delisteds := make([]bool, len(users))
	reasons := make([]string, len(users))
	for i, u := range users {
		createdAts[i] = u.CreatedAt
		userIDs[i] = u.UserID
		delisteds[i] = u.Delisted
		reasons[i] = u.Reason
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_delist_statuses (created_at, user_id, delisted, reason)
		SELECT
		  unnest($1::timestamptz[]),
		  unnest($2::int[]),
		  unnest($3::bool[]),
		  unnest($4::delist_user_reason[])
		ON CONFLICT DO NOTHING
	`, createdAts, userIDs, delisteds, reasons)
	return err
}

func insertTrackDelistStatuses(ctx context.Context, tx pgx.Tx, tracks []delistTrack) error {
	if len(tracks) == 0 {
		return nil
	}
	createdAts := make([]string, len(tracks))
	trackIDs := make([]int64, len(tracks))
	ownerIDs := make([]int64, len(tracks))
	cids := make([]string, len(tracks))
	delisteds := make([]bool, len(tracks))
	reasons := make([]string, len(tracks))
	for i, t := range tracks {
		createdAts[i] = t.CreatedAt
		trackIDs[i] = t.TrackID
		ownerIDs[i] = t.OwnerID
		cids[i] = t.TrackCID
		delisteds[i] = t.Delisted
		reasons[i] = t.Reason
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO track_delist_statuses (created_at, track_id, owner_id, track_cid, delisted, reason)
		SELECT
		  unnest($1::timestamptz[]),
		  unnest($2::int[]),
		  unnest($3::int[]),
		  unnest($4::text[]),
		  unnest($5::bool[]),
		  unnest($6::delist_track_reason[])
		ON CONFLICT DO NOTHING
	`, createdAts, trackIDs, ownerIDs, cids, delisteds, reasons)
	return err
}

// selectExistingIDs runs a SELECT that returns one int64 per row and returns
// the result as a set.
func selectExistingIDs(ctx context.Context, tx pgx.Tx, sql string, ids []int64) (map[int64]struct{}, error) {
	if len(ids) == 0 {
		return map[int64]struct{}{}, nil
	}
	rows, err := tx.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]struct{}, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

// parseDelistTimestamp accepts the two formats apps' notifier emits:
//
//	"YYYY-MM-DD HH:MM:SS.ffffff+00"
//	"YYYY-MM-DD HH:MM:SS+00"
//
// plus the RFC3339 forms in case the notifier upgrades.
func parseDelistTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999+00",
		"2006-01-02 15:04:05+00",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp: %q", s)
}

