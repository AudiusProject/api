package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	connect "connectrpc.com/connect"
	ethv1 "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"go.uber.org/zap"
)

// RepairTrackCidsJob backfills tracks whose audio finished transcoding but
// whose track_cid never made it onto the track entity.
//
// track_cid is taken verbatim from the uploader's metadata at index time. The
// client is supposed to poll the content node until the transcode finishes and
// then include the resulting cid when it writes the track, but when that
// handshake falls through the track is indexed with a NULL track_cid. The audio
// is fine and sitting on the content node - there is simply no cid on the row
// pointing at it, so nothing can be signed and nothing can be played. The
// track is silently dead: it looks normal, collects favorites and reposts, and
// never accumulates a single play.
//
// The content node is the authoritative source for what it produced, and it
// still holds the upload record keyed by the track's audio_upload_id. This job
// reconciles the gap from there, the same way RepairAudioAnalysesJob recovers
// bpm / musical_key.
//
// Each pass:
//  1. Selects up to trackCidBatchSize current, undeleted tracks with a NULL
//     track_cid and an audio_upload_id to look up (newest first).
//  2. Picks up to trackCidMaxNodes random registered content nodes.
//  3. Queries nodes for each upload record until trackCidQuorum of them agree
//     on the same transcoded cid.
//  4. Writes track_cid, committing per track.
type RepairTrackCidsJob struct {
	pool       database.DbPool
	logger     *zap.Logger
	sdk        *sdk.OpenAudioSDK
	httpClient *http.Client

	mutex     sync.Mutex
	isRunning bool
}

const (
	// trackCidBatchSize matches RepairAudioAnalysesJob's batch.
	trackCidBatchSize = 1000
	// trackCidMaxNodes bounds how many nodes a single pass will ask.
	trackCidMaxNodes = 5
	// trackCidNodeTimeout matches RepairAudioAnalysesJob's per-request budget.
	trackCidNodeTimeout = 5 * time.Second
	// trackCidQuorum is how many distinct content nodes must report the same
	// cid before it is written.
	//
	// This is a higher bar than the bpm / musical_key repair asks for, and
	// deliberately so: those fill in a display field, while track_cid decides
	// which bytes every listener receives for this track. Upload records are
	// replicated across mirrors, so agreement is cheap to obtain and means a
	// single misbehaving or out-of-date node cannot repoint a track's audio on
	// its own. A track that cannot reach quorum is left alone for the next pass
	// rather than repaired from one node's word.
	trackCidQuorum = 2
)

func NewRepairTrackCidsJob(cfg config.Config, pool database.DbPool, oaSDK *sdk.OpenAudioSDK) *RepairTrackCidsJob {
	return &RepairTrackCidsJob{
		pool:       pool,
		logger:     logging.NewZapLogger(cfg).Named("RepairTrackCidsJob"),
		sdk:        oaSDK,
		httpClient: &http.Client{Timeout: trackCidNodeTimeout},
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *RepairTrackCidsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *RepairTrackCidsJob {
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
func (j *RepairTrackCidsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *RepairTrackCidsJob) run(ctx context.Context) error {
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

	tracks, err := j.queryTracks(ctx)
	if err != nil {
		return fmt.Errorf("query tracks: %w", err)
	}
	if len(tracks) == 0 {
		return nil
	}

	nodes, err := j.selectContentNodes(ctx)
	if err != nil {
		return fmt.Errorf("select content nodes: %w", err)
	}
	if len(nodes) < trackCidQuorum {
		j.logger.Warn("not enough content nodes to reach quorum; skipping pass",
			zap.Int("nodes", len(nodes)), zap.Int("quorum", trackCidQuorum))
		return nil
	}

	repaired := 0
	for _, t := range tracks {
		ok, err := j.repairTrackCid(ctx, t, nodes)
		if err != nil {
			j.logger.Error("repairing track cid failed",
				zap.Int64("track_id", t.TrackID), zap.Error(err))
			continue
		}
		if ok {
			repaired++
		}
	}

	j.logger.Info("Repaired track cids",
		zap.Int("candidates", len(tracks)),
		zap.Int("repaired", repaired))
	return nil
}

type cidlessTrack struct {
	TrackID       int64
	AudioUploadID string
}

// queryTracks selects tracks that have no playable audio pointer but do carry
// the upload id needed to find one. Deleted tracks are skipped - their audio is
// meant to be unreachable - as are stems and rows with no audio_upload_id,
// which are legacy uploads with nothing to look up.
func (j *RepairTrackCidsJob) queryTracks(ctx context.Context) ([]cidlessTrack, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT track_id, audio_upload_id
		FROM tracks
		WHERE is_current = true
		  AND is_delete = false
		  AND track_cid IS NULL
		  AND audio_upload_id IS NOT NULL
		  AND audio_upload_id <> ''
		ORDER BY created_at DESC
		LIMIT $1
	`, trackCidBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []cidlessTrack
	for rows.Next() {
		var t cidlessTrack
		if err := rows.Scan(&t.TrackID, &t.AudioUploadID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// selectContentNodes takes up to trackCidMaxNodes random registered
// content-node endpoints. Mirrors RepairAudioAnalysesJob.selectContentNodes.
func (j *RepairTrackCidsJob) selectContentNodes(ctx context.Context) ([]string, error) {
	resp, err := j.sdk.Eth.GetRegisteredEndpoints(ctx, connect.NewRequest(&ethv1.GetRegisteredEndpointsRequest{}))
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("GetRegisteredEndpoints returned nil response")
	}

	var endpoints []string
	for _, node := range resp.Msg.Endpoints {
		if node.ServiceType != "content-node" {
			continue
		}
		ep := strings.TrimRight(strings.ToLower(strings.TrimSpace(node.Endpoint)), "/")
		if ep != "" {
			endpoints = append(endpoints, ep)
		}
	}

	rand.Shuffle(len(endpoints), func(a, b int) {
		endpoints[a], endpoints[b] = endpoints[b], endpoints[a]
	})
	if len(endpoints) > trackCidMaxNodes {
		endpoints = endpoints[:trackCidMaxNodes]
	}
	return endpoints, nil
}

// uploadRecord is the subset of mediorum's /uploads/:id payload this job needs.
type uploadRecord struct {
	Status           string            `json:"status"`
	TranscodeResults map[string]string `json:"results"`
}

// transcodedCid returns the 320kbps cid of a finished transcode, or "" when the
// upload has not produced one yet.
func (u uploadRecord) transcodedCid() string {
	if u.Status != "done" {
		return ""
	}
	return strings.TrimSpace(u.TranscodeResults["320"])
}

// repairTrackCid asks content nodes for one track's upload record and writes
// the transcoded cid once trackCidQuorum nodes agree on it. Returns true when
// the track was repaired.
func (j *RepairTrackCidsJob) repairTrackCid(ctx context.Context, t cidlessTrack, nodes []string) (bool, error) {
	votes := make(map[string]int, 2)
	for _, node := range nodes {
		cid, ok := j.fetchTranscodedCid(ctx, node, t.AudioUploadID)
		if !ok || cid == "" {
			// Transport error, no record, or a transcode that has not finished:
			// nothing to count. Ask the next node.
			continue
		}

		votes[cid]++
		if votes[cid] < trackCidQuorum {
			continue
		}

		if err := j.applyTrackCid(ctx, t.TrackID, cid); err != nil {
			return false, fmt.Errorf("update track %d: %w", t.TrackID, err)
		}
		j.logger.Info("repaired track cid",
			zap.Int64("track_id", t.TrackID),
			zap.String("audio_upload_id", t.AudioUploadID),
			zap.String("track_cid", cid))
		return true, nil
	}

	if len(votes) > 1 {
		// Nodes disagreed about what this upload transcoded to. Never guess
		// which one is right; leave the row alone and say so loudly.
		j.logger.Warn("content nodes disagree on transcoded cid; leaving track unrepaired",
			zap.Int64("track_id", t.TrackID),
			zap.String("audio_upload_id", t.AudioUploadID),
			zap.Any("votes", votes))
	}
	return false, nil
}

// fetchTranscodedCid GETs one node's upload record. ok=false signals a
// transport/non-2xx error; ok=true with an empty cid means the node answered
// but the upload has not finished transcoding.
func (j *RepairTrackCidsJob) fetchTranscodedCid(ctx context.Context, node, uploadID string) (cid string, ok bool) {
	endpoint := fmt.Sprintf("%s/uploads/%s", node, uploadID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", false
	}
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return "", false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	var parsed uploadRecord
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false
	}
	return parsed.transcodedCid(), true
}

// applyTrackCid writes the cid, re-checking that the row is still cidless so a
// concurrent indexer write of the real metadata always wins.
func (j *RepairTrackCidsJob) applyTrackCid(ctx context.Context, trackID int64, cid string) error {
	_, err := j.pool.Exec(ctx, `
		UPDATE tracks SET track_cid = $2
		WHERE track_id = $1 AND is_current = true AND track_cid IS NULL
	`, trackID, cid)
	return err
}
