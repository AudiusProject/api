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

// RepairAudioAnalysesJob backfills missing track bpm / musical_key by polling
// content nodes for their stored audio-analysis results. It is a direct port
// of apps' src/tasks/repair_audio_analyses.py celery task.
//
// The entity-manager indexer only records bpm / musical_key when the uploader
// supplies them in track metadata (or marks them custom). Tracks whose
// analysis finished after indexing — or which were indexed before the indexer
// learned to persist these fields — are left with NULL bpm / musical_key. This
// recurring job reconciles those gaps from the authoritative source: the
// content node that performed the analysis.
//
// Each pass:
//  1. Selects up to repairBatchSize current tracks missing a non-custom bpm or
//     musical_key, with fewer than 3 prior analysis errors and a streamable
//     track_cid (newest first).
//  2. Picks up to repairMaxNodes random registered content nodes.
//  3. For each track, queries a node for the analysis (modern uploads endpoint
//     when audio_upload_id is set, else the legacy blob-analysis endpoint),
//     falling back to the next node on transport error.
//  4. Fills in any missing+valid bpm / musical_key and syncs the stored error
//     count, committing per track.
type RepairAudioAnalysesJob struct {
	pool       database.DbPool
	logger     *zap.Logger
	sdk        *sdk.OpenAudioSDK
	httpClient *http.Client

	mutex     sync.Mutex
	isRunning bool
}

const (
	// repairBatchSize mirrors apps' BATCH_SIZE.
	repairBatchSize = 1000
	// repairMaxNodes mirrors apps' random.sample(endpoints, min(5, ...)).
	repairMaxNodes = 5
	// repairNodeTimeout mirrors apps' requests.get(..., timeout=5).
	repairNodeTimeout = 5 * time.Second
	// repairMaxErrorCount mirrors apps' audio_analysis_error_count < 3 filter.
	repairMaxErrorCount = 3
)

func NewRepairAudioAnalysesJob(cfg config.Config, pool database.DbPool, oaSDK *sdk.OpenAudioSDK) *RepairAudioAnalysesJob {
	return &RepairAudioAnalysesJob{
		pool:       pool,
		logger:     logging.NewZapLogger(cfg).Named("RepairAudioAnalysesJob"),
		sdk:        oaSDK,
		httpClient: &http.Client{Timeout: repairNodeTimeout},
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *RepairAudioAnalysesJob) ScheduleEvery(ctx context.Context, interval time.Duration) *RepairAudioAnalysesJob {
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
func (j *RepairAudioAnalysesJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *RepairAudioAnalysesJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		// apps guards with a redis lock + non-blocking acquire; the in-process
		// mutex is the equivalent here (single indexer process).
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
	if len(nodes) == 0 {
		j.logger.Warn("no content nodes available; skipping pass")
		return nil
	}

	updated := 0
	for _, t := range tracks {
		ok, err := j.repairTrack(ctx, t, nodes)
		if err != nil {
			j.logger.Error("repairing track failed",
				zap.Int64("track_id", t.TrackID), zap.Error(err))
			continue
		}
		if ok {
			updated++
		}
	}

	j.logger.Info("Repaired audio analyses",
		zap.Int("candidates", len(tracks)),
		zap.Int("updated", updated),
		zap.Int64("last_track_id", tracks[len(tracks)-1].TrackID))
	return nil
}

type repairTrack struct {
	TrackID            int64
	TrackCID           string
	AudioUploadID      string
	Bpm                *float64
	IsCustomBpm        bool
	MusicalKey         *string
	IsCustomMusicalKey bool
	ErrorCount         int
}

// queryTracks mirrors apps' query_tracks: current tracks missing a non-custom
// bpm or musical_key, error count < 3, streamable (track_cid not null), and not
// a podcast/audiobook, newest first.
func (j *RepairAudioAnalysesJob) queryTracks(ctx context.Context) ([]repairTrack, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT track_id, track_cid, COALESCE(audio_upload_id, ''),
		       bpm, COALESCE(is_custom_bpm, false),
		       musical_key, COALESCE(is_custom_musical_key, false),
		       audio_analysis_error_count
		FROM tracks
		WHERE is_current = true
		  AND (
		        (musical_key IS NULL AND is_custom_musical_key = false)
		     OR (bpm IS NULL AND is_custom_bpm = false)
		  )
		  AND audio_analysis_error_count < $1
		  AND track_cid IS NOT NULL
		  AND genre NOT IN ('Podcasts', 'Podcast', 'Audiobooks')
		ORDER BY created_at DESC
		LIMIT $2
	`, repairMaxErrorCount, repairBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repairTrack
	for rows.Next() {
		var t repairTrack
		if err := rows.Scan(
			&t.TrackID, &t.TrackCID, &t.AudioUploadID,
			&t.Bpm, &t.IsCustomBpm,
			&t.MusicalKey, &t.IsCustomMusicalKey,
			&t.ErrorCount,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// selectContentNodes mirrors apps' select_content_nodes: take up to
// repairMaxNodes random registered content-node endpoints (lowercased, trailing
// slash trimmed). apps samples from redis-cached *healthy* nodes; we don't have
// that cache here, so we sample from all registered content nodes and rely on
// the per-request timeout + next-node fallback to skip unhealthy ones.
func (j *RepairAudioAnalysesJob) selectContentNodes(ctx context.Context) ([]string, error) {
	resp, err := j.sdk.Eth.GetRegisteredEndpoints(ctx, connect.NewRequest(&ethv1.GetRegisteredEndpointsRequest{}))
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("GetRegisteredEndpoints returned nil response")
	}

	var endpoints []string
	for _, node := range resp.Msg.Endpoints {
		// Content nodes serve the audio-analysis endpoints (mediorum storage).
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
	if len(endpoints) > repairMaxNodes {
		endpoints = endpoints[:repairMaxNodes]
	}
	return endpoints, nil
}

// analysisResult is the shape served by mediorum for both the modern uploads
// endpoint and the legacy blob-analysis endpoint. The legacy python content
// node used capitalized keys (Key/BPM); we accept both for parity.
type analysisResult struct {
	BPM    float64 `json:"bpm"`
	BPMCap float64 `json:"BPM"`
	Key    string  `json:"key"`
	KeyCap string  `json:"Key"`
}

func (a analysisResult) bpm() float64 {
	if a.BPM != 0 {
		return a.BPM
	}
	return a.BPMCap
}

func (a analysisResult) key() string {
	if a.Key != "" {
		return a.Key
	}
	return a.KeyCap
}

// modernAnalysis matches mediorum's /uploads/:id payload.
type modernAnalysis struct {
	Results    *analysisResult `json:"audio_analysis_results"`
	ErrorCount int             `json:"audio_analysis_error_count"`
}

// legacyAnalysis matches mediorum's /tracks/legacy/:cid/analysis payload.
type legacyAnalysis struct {
	Results    *analysisResult `json:"results"`
	ErrorCount int             `json:"error_count"`
}

// repairTrack queries content nodes for one track's analysis and applies any
// missing+valid bpm / musical_key plus the latest error count. Mirrors the
// per-track body of apps' repair(). Returns true when the track was updated.
func (j *RepairAudioAnalysesJob) repairTrack(ctx context.Context, t repairTrack, nodes []string) (bool, error) {
	if t.TrackCID == "" {
		// Only analyze streamable tracks.
		return false, nil
	}
	legacy := t.AudioUploadID == ""

	for _, node := range nodes {
		var endpoint string
		if legacy {
			endpoint = fmt.Sprintf("%s/tracks/legacy/%s/analysis", node, t.TrackCID)
		} else {
			endpoint = fmt.Sprintf("%s/uploads/%s", node, t.AudioUploadID)
		}

		result, errorCount, ok := j.fetchAnalysis(ctx, endpoint, legacy)
		if !ok {
			// Transport/non-2xx error: fall back to the next node.
			continue
		}

		var key string
		var bpm float64
		if result != nil {
			key = result.key()
			bpm = result.bpm()
		}

		// Build the update: fill missing+valid fields and sync error count.
		setMusicalKey := key != "" && t.MusicalKey == nil && !t.IsCustomMusicalKey && isValidMusicalKey(key)
		setBpm := bpm != 0 && t.Bpm == nil && !t.IsCustomBpm && isValidBpm(bpm)
		setErrorCount := errorCount != t.ErrorCount

		if setMusicalKey || setBpm || setErrorCount {
			if err := j.applyUpdate(ctx, t.TrackID, setMusicalKey, key, setBpm, bpm, setErrorCount, errorCount); err != nil {
				return false, fmt.Errorf("update track %d: %w", t.TrackID, err)
			}
		}

		if errorCount >= repairMaxErrorCount {
			j.logger.Warn("track failed audio analysis >= 3 times",
				zap.Int64("track_id", t.TrackID),
				zap.String("track_cid", t.TrackCID),
				zap.String("audio_upload_id", t.AudioUploadID))
		}

		// apps breaks after the first node that responds (success or empty).
		return setMusicalKey || setBpm || setErrorCount, nil
	}

	return false, nil
}

// fetchAnalysis GETs the endpoint and parses the analysis. ok=false signals a
// transport/non-2xx error (caller should try the next node); ok=true with a nil
// result means the node responded but had no analysis yet.
func (j *RepairAudioAnalysesJob) fetchAnalysis(ctx context.Context, endpoint string, legacy bool) (result *analysisResult, errorCount int, ok bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, false
	}
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return nil, 0, false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, false
	}

	if legacy {
		var parsed legacyAnalysis
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, 0, false
		}
		return parsed.Results, parsed.ErrorCount, true
	}
	var parsed modernAnalysis
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, 0, false
	}
	return parsed.Results, parsed.ErrorCount, true
}

// applyUpdate writes only the fields that changed, mirroring apps' selective
// setattr + commit. Each track commits independently.
func (j *RepairAudioAnalysesJob) applyUpdate(ctx context.Context, trackID int64, setKey bool, key string, setBpm bool, bpm float64, setErr bool, errorCount int) error {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	args = append(args, trackID)
	if setKey {
		args = append(args, key)
		sets = append(sets, fmt.Sprintf("musical_key = $%d", len(args)))
	}
	if setBpm {
		args = append(args, bpm)
		sets = append(sets, fmt.Sprintf("bpm = $%d", len(args)))
	}
	if setErr {
		args = append(args, errorCount)
		sets = append(sets, fmt.Sprintf("audio_analysis_error_count = $%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	sql := fmt.Sprintf(
		"UPDATE tracks SET %s WHERE track_id = $1 AND is_current = true",
		strings.Join(sets, ", "))
	_, err := j.pool.Exec(ctx, sql, args...)
	return err
}

// isValidBpm mirrors apps' valid_bpm: a float strictly between 0 and 999.
func isValidBpm(bpm float64) bool {
	return bpm > 0 && bpm < 999
}
