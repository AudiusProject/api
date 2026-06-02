package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

// ReconcileAggregatesJob is the drift backstop for aggregate counts. The
// per-action triggers (handle_repost/handle_save/handle_follow) maintain the
// counts on the hot path with O(1) deltas; this job periodically recomputes
// them from the source tables and corrects any divergence. Ported from
// discovery-provider's update_aggregates.py (celery, every 10 minutes).
//
// It only writes the count columns and is column-disjoint from
// UpdateAggregatesJob, which owns aggregate_user.score.
type ReconcileAggregatesJob struct {
	pool   database.DbPool
	logger *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewReconcileAggregatesJob(cfg config.Config, pool database.DbPool) *ReconcileAggregatesJob {
	return &ReconcileAggregatesJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("ReconcileAggregatesJob"),
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *ReconcileAggregatesJob) ScheduleEvery(ctx context.Context, interval time.Duration) *ReconcileAggregatesJob {
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
func (j *ReconcileAggregatesJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *ReconcileAggregatesJob) run(ctx context.Context) error {
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

	for _, step := range []struct {
		name  string
		query string
	}{
		{"aggregate_user", reconcileAggregateUserQuery},
		{"aggregate_track", reconcileAggregateTrackQuery},
		{"aggregate_playlist", reconcileAggregatePlaylistQuery},
	} {
		res, err := j.pool.Exec(ctx, step.query)
		if err != nil {
			return fmt.Errorf("reconcile %s: %w", step.name, err)
		}
		if n := res.RowsAffected(); n > 0 {
			j.logger.Info("Reconciled aggregates", zap.String("table", step.name), zap.Int64("corrected", n))
		}
	}
	return nil
}

const reconcileAggregateUserQuery = `
with user_repost as (
  select user_id, count(*) as repost_count
  from reposts r
  where r.is_current is true and r.is_delete is false
  group by user_id
),
user_save as (
  select user_id, count(*) as track_save_count
  from saves s
  where s.is_current is true and s.is_delete is false and s.save_type = 'track'
  group by user_id
),
user_following as (
  select follower_user_id as user_id, count(*) as following_count
  from follows
  where is_current = true and is_delete = false
  group by follower_user_id
),
user_follower as (
  select followee_user_id as user_id, count(*) as follower_count
  from follows
  where is_current = true and is_delete = false
  group by followee_user_id
),
user_album as (
  select playlist_owner_id as user_id, count(*) as album_count
  from playlists p
  where p.is_album is true and p.is_current is true and p.is_delete is false and p.is_private is false
  group by playlist_owner_id
),
user_playlist as (
  select playlist_owner_id as user_id, count(*) as playlist_count
  from playlists p
  where p.is_album is false and p.is_current is true and p.is_delete is false and p.is_private is false
  group by playlist_owner_id
),
user_track as (
  select owner_id as user_id, count(*) as track_count
  from tracks t
  where t.is_current is true and t.is_delete is false and t.is_unlisted is false
    and t.is_available is true and t.stem_of is null
  group by owner_id
),
genre_counts as (
  select owner_id as user_id, genre, count(*) as count, max(created_at) as latest_upload
  from tracks t
  where t.is_current is true and t.is_delete is false and t.is_unlisted is false
    and t.is_available is true and t.stem_of is null
  group by genre, owner_id
),
ranked_genres as (
  select user_id, genre, count,
    rank() over (partition by user_id order by count desc, latest_upload desc) as genre_rank
  from genre_counts
),
new_aggregate_user as (
  select
    ap.user_id,
    coalesce(ut.track_count, 0) as track_count,
    coalesce(up.playlist_count, 0) as playlist_count,
    coalesce(ua.album_count, 0) as album_count,
    coalesce(ufollower.follower_count, 0) as follower_count,
    coalesce(ufollowing.following_count, 0) as following_count,
    coalesce(ur.repost_count, 0) as repost_count,
    coalesce(us.track_save_count, 0) as track_save_count,
    rg.genre as dominant_genre,
    rg.count as dominant_genre_count
  from aggregate_user ap
    left join user_track ut on ap.user_id = ut.user_id
    left join user_playlist up on ap.user_id = up.user_id
    left join user_album ua on ap.user_id = ua.user_id
    left join user_follower ufollower on ap.user_id = ufollower.user_id
    left join user_following ufollowing on ap.user_id = ufollowing.user_id
    left join user_save us on ap.user_id = us.user_id
    left join user_repost ur on ap.user_id = ur.user_id
    left join ranked_genres rg on ap.user_id = rg.user_id and rg.genre_rank = 1
)
update aggregate_user au
set
  track_count = nau.track_count,
  playlist_count = nau.playlist_count,
  album_count = nau.album_count,
  follower_count = nau.follower_count,
  following_count = nau.following_count,
  repost_count = nau.repost_count,
  track_save_count = nau.track_save_count,
  dominant_genre = nau.dominant_genre,
  dominant_genre_count = nau.dominant_genre_count
from new_aggregate_user nau
where au.user_id = nau.user_id
  and (
    au.track_count != nau.track_count
    or au.playlist_count != nau.playlist_count
    or au.album_count != nau.album_count
    or au.follower_count != nau.follower_count
    or au.following_count != nau.following_count
    or au.repost_count != nau.repost_count
    or au.track_save_count != nau.track_save_count
    or au.dominant_genre is distinct from nau.dominant_genre
    or au.dominant_genre_count is distinct from nau.dominant_genre_count
  );
`

const reconcileAggregateTrackQuery = `
with track_saves as (
  select save_item_id, count(*) as save_count
  from saves s
  where s.is_current is true and s.is_delete is false and s.save_type = 'track'
  group by save_item_id
),
track_reposts as (
  select repost_item_id, count(*) as repost_count
  from reposts r
  where r.is_current is true and r.is_delete is false and r.repost_type = 'track'
  group by repost_item_id
),
track_comments as (
  select entity_id as comment_entity_id, count(*) as comment_count
  from comments c
  where c.is_delete is false and c.is_visible is true and c.entity_type = 'Track'
  group by comment_entity_id
),
new_aggregate_track as (
  select
    ap.track_id,
    coalesce(ps.save_count, 0) as save_count,
    coalesce(pr.repost_count, 0) as repost_count,
    coalesce(pc.comment_count, 0) as comment_count
  from aggregate_track ap
    left join track_saves ps on ap.track_id = ps.save_item_id
    left join track_reposts pr on ap.track_id = pr.repost_item_id
    left join track_comments pc on ap.track_id = pc.comment_entity_id
)
update aggregate_track at
set
  save_count = nat.save_count,
  repost_count = nat.repost_count,
  comment_count = nat.comment_count
from new_aggregate_track nat
where at.track_id = nat.track_id
  and (
    at.save_count != nat.save_count
    or at.repost_count != nat.repost_count
    or at.comment_count != nat.comment_count
  );
`

const reconcileAggregatePlaylistQuery = `
with playlist_saves as (
  select save_item_id, count(*) as save_count
  from saves s
  where s.is_current is true and s.is_delete is false
    and (s.save_type = 'playlist' or s.save_type = 'album')
  group by save_item_id
),
playlist_reposts as (
  select repost_item_id, count(*) as repost_count
  from reposts r
  where r.is_current is true and r.is_delete is false
    and (r.repost_type = 'playlist' or r.repost_type = 'album')
  group by repost_item_id
),
new_aggregate_playlist as (
  select
    ap.playlist_id,
    coalesce(ps.save_count, 0) as save_count,
    coalesce(pr.repost_count, 0) as repost_count
  from aggregate_playlist ap
    left join playlist_saves ps on ap.playlist_id = ps.save_item_id
    left join playlist_reposts pr on ap.playlist_id = pr.repost_item_id
)
update aggregate_playlist ap
set
  save_count = nap.save_count,
  repost_count = nap.repost_count
from new_aggregate_playlist nap
where ap.playlist_id = nap.playlist_id
  and (ap.save_count != nap.save_count or ap.repost_count != nap.repost_count);
`
