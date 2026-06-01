package jobs

import (
	"context"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/jobs/challenges"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

// IndexChallengesJob reconciles derived challenge state from source tables.
//
// Mirrors apps' index_challenges celery task in role but not implementation:
// where apps drains a Redis queue of dispatched events, we reconcile derived
// state from source tables. See package docs in jobs/challenges/processor.go.
//
// Scheduling model: rather than one tick that runs every processor back-to-back
// (which made a slow processor delay the cheap real-time ones and bunched all
// the DB work into one burst), each challengeGroup runs on its own cadence in
// its own goroutine. Most groups hold a single processor; processors with an
// ordering dependency (the play-count milestones, where p2 requires p1) share a
// group so they run in sequence within one tick. Independent goroutines plus a
// small startup stagger keep the groups from stomping on each other.
type IndexChallengesJob struct {
	pool   database.DbPool
	logger *zap.Logger
	groups []challengeGroup
}

// challengeGroup is one or more processors that run together on a shared
// cadence. Processors in a group run sequentially in registration order (so an
// intra-group dependency like play-count p1→p2→p3 is honored); separate groups
// run concurrently on their own goroutines.
type challengeGroup struct {
	name     string
	interval time.Duration
	procs    []challenges.Processor
}

const (
	// Checkpoint-incremental processors: each tick is an index range scan over
	// rows changed since the last checkpoint, so an idle tick is nearly free and
	// a just-earned challenge is picked up on the next tick. Run near-real-time.
	challengeFastInterval = 30 * time.Second

	// Bounded full-scan processors: rescan a constrained source set (verified
	// artists, top-N trending, current contest events) each tick. Not free, but
	// cheap enough that a couple of minutes keeps them timely without hot-looping.
	challengeMediumInterval = 2 * time.Minute

	// Trending reward processors only mint rows once per week (Friday UTC); off
	// that window each tick is a guard check, so a slow cadence is plenty.
	challengeSlowInterval = 10 * time.Minute
)

// NewIndexChallengesJob constructs the umbrella job with all processors wired
// into cadence groups.
func NewIndexChallengesJob(cfg config.Config, pool database.DbPool) *IndexChallengesJob {
	groups := []challengeGroup{
		// Fast / checkpoint-incremental — one group per processor at 30s.
		{"track_upload", challengeFastInterval, []challenges.Processor{&challenges.TrackUploadProcessor{}}},
		{"first_playlist", challengeFastInterval, []challenges.Processor{&challenges.FirstPlaylistProcessor{}}},
		{"profile_completion", challengeFastInterval, []challenges.Processor{&challenges.ProfileCompletionProcessor{}}},
		{"listen_streak", challengeFastInterval, []challenges.Processor{&challenges.ListenStreakProcessor{}}},
		{"cosign", challengeFastInterval, []challenges.Processor{&challenges.CosignProcessor{}}},
		{"audio_matching_buyer", challengeFastInterval, []challenges.Processor{challenges.NewAudioMatchingBuyerProcessor()}},
		{"audio_matching_seller", challengeFastInterval, []challenges.Processor{challenges.NewAudioMatchingSellerProcessor()}},
		{"mobile_install", challengeFastInterval, []challenges.Processor{&challenges.MobileInstallProcessor{}}},
		{"referral", challengeFastInterval, []challenges.Processor{challenges.NewReferralProcessor()}},
		{"verified_referral", challengeFastInterval, []challenges.Processor{challenges.NewVerifiedReferralProcessor()}},
		{"referred", challengeFastInterval, []challenges.Processor{&challenges.ReferredProcessor{}}},

		// Medium / bounded full-scan — 2m each.
		{"profile_verified", challengeMediumInterval, []challenges.Processor{&challenges.ProfileVerifiedProcessor{}}},
		{"first_weekly_comment", challengeMediumInterval, []challenges.Processor{&challenges.FirstWeeklyCommentProcessor{}}},
		{"comment_pin", challengeMediumInterval, []challenges.Processor{&challenges.CommentPinProcessor{}}},
		{"tastemaker", challengeMediumInterval, []challenges.Processor{&challenges.TastemakerProcessor{}}},
		{"remix_contest_winner", challengeMediumInterval, []challenges.Processor{&challenges.RemixContestWinnerProcessor{}}},
		// Play-count milestones share a group: p2 requires p1 complete and p3
		// requires p2, so they must run in order within a single tick.
		{"play_count_milestones", challengeMediumInterval, []challenges.Processor{
			challenges.NewPlayCount250Processor(),
			challenges.NewPlayCount1000Processor(),
			challenges.NewPlayCount10000Processor(),
		}},

		// Slow / weekly reward processors — 10m each.
		{"trending_track", challengeSlowInterval, []challenges.Processor{challenges.NewTrendingTrackProcessor()}},
		{"trending_underground", challengeSlowInterval, []challenges.Processor{challenges.NewTrendingUndergroundProcessor()}},
		{"trending_playlist", challengeSlowInterval, []challenges.Processor{challenges.NewTrendingPlaylistProcessor()}},
	}

	return &IndexChallengesJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("IndexChallengesJob"),
		groups: groups,
	}
}

// Schedule launches one goroutine per cadence group, each running its
// processors on the group's interval until the context is cancelled. Returns
// the job so callers can chain.
func (j *IndexChallengesJob) Schedule(ctx context.Context) *IndexChallengesJob {
	for i := range j.groups {
		// Stagger startup so the groups don't all fire their first run at once.
		offset := time.Duration(i) * time.Second
		go j.runGroupLoop(ctx, j.groups[i], offset)
	}
	return j
}

// runGroupLoop runs one group: an initial (staggered) pass, then a pass on each
// tick. Because a group runs in a single goroutine, its passes can never
// overlap — if a pass overruns the interval the Ticker drops the missed ticks.
func (j *IndexChallengesJob) runGroupLoop(ctx context.Context, g challengeGroup, offset time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(offset):
	}

	j.runGroup(ctx, g)

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			j.logger.Info("Challenge group shutting down", zap.String("group", g.name))
			return
		case <-ticker.C:
			j.runGroup(ctx, g)
		}
	}
}

// runGroup runs every processor in a group in order, each in its own
// transaction. A failure in one processor doesn't stop the others.
func (j *IndexChallengesJob) runGroup(ctx context.Context, g challengeGroup) {
	start := time.Now()
	for _, p := range g.procs {
		if err := j.runProcessor(ctx, p); err != nil {
			j.logger.Error("processor failed",
				zap.String("group", g.name),
				zap.String("challenge_id", p.ChallengeID()),
				zap.Error(err))
			// Continue — one bad processor shouldn't kill the rest.
		}
	}
	j.logger.Info("Reconciled challenge group",
		zap.String("group", g.name),
		zap.Int("processors", len(g.procs)),
		zap.Duration("duration", time.Since(start)))
}

// runProcessor runs a single processor in its own transaction.
func (j *IndexChallengesJob) runProcessor(ctx context.Context, p challenges.Processor) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := p.Reconcile(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
