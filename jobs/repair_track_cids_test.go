package jobs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTranscodedCid(t *testing.T) {
	// A finished transcode hands back its 320kbps cid.
	done := uploadRecord{Status: "done", TranscodeResults: map[string]string{"320": "QmDone"}}
	assert.Equal(t, "QmDone", done.transcodedCid())

	// An upload still in flight has nothing to offer, even if a partial result
	// is already present - writing that would point the track at a half-built
	// blob.
	busy := uploadRecord{Status: "busy", TranscodeResults: map[string]string{"320": "QmPartial"}}
	assert.Equal(t, "", busy.transcodedCid())

	// A finished upload with no 320 result is not a repair candidate.
	empty := uploadRecord{Status: "done", TranscodeResults: map[string]string{}}
	assert.Equal(t, "", empty.transcodedCid())

	whitespace := uploadRecord{Status: "done", TranscodeResults: map[string]string{"320": "  QmPadded  "}}
	assert.Equal(t, "QmPadded", whitespace.transcodedCid())
}

func newTrackCidJob(pool database.DbPool) *RepairTrackCidsJob {
	return &RepairTrackCidsJob{
		pool:       pool,
		logger:     zap.NewNop(),
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}
}

func TestFetchTranscodedCid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/uploads/up-1", r.URL.Path)
		w.Write([]byte(`{"status":"done","results":{"320":"QmGood"}}`))
	}))
	defer srv.Close()

	cid, ok := newTrackCidJob(nil).fetchTranscodedCid(context.Background(), srv.URL, "up-1")
	require.True(t, ok)
	assert.Equal(t, "QmGood", cid)
}

func TestFetchTranscodedCidNon2xxIsNotOk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, ok := newTrackCidJob(nil).fetchTranscodedCid(context.Background(), srv.URL, "missing")
	assert.False(t, ok, "a node that does not hold the upload must not count as a vote")
}

func TestFetchTranscodedCidUnfinishedTranscode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"busy","results":{}}`))
	}))
	defer srv.Close()

	cid, ok := newTrackCidJob(nil).fetchTranscodedCid(context.Background(), srv.URL, "up-1")
	assert.True(t, ok, "the node answered")
	assert.Equal(t, "", cid, "but has no cid to offer yet")
}

// nodeServing returns a content node stub that reports the given upload record
// JSON, and counts how many times it was asked.
func nodeServing(t *testing.T, body string, calls *int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func seedCidlessTrack(t *testing.T, pool *pgxpool.Pool, uploadID string) {
	t.Helper()
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01", "handle": "testuser1"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "No Cid", "audio_upload_id": uploadID},
		},
	})
}

func trackCidOf(t *testing.T, pool *pgxpool.Pool, trackID int64) *string {
	t.Helper()
	var cid *string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT track_cid FROM tracks WHERE track_id = $1", trackID).Scan(&cid))
	return cid
}

// The whole point of the job: an upload that transcoded fine but was indexed
// without its cid gets its audio pointer back.
func TestRepairTrackCidsJob_RepairsOnQuorum(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	seedCidlessTrack(t, pool, "up-1")

	body := `{"status":"done","results":{"320":"QmAgreed"}}`
	calls := 0
	nodes := []string{
		nodeServing(t, body, &calls),
		nodeServing(t, body, &calls),
		nodeServing(t, body, &calls),
	}

	job := newTrackCidJob(pool)
	repaired, err := job.repairTrackCid(context.Background(),
		cidlessTrack{TrackID: 100, AudioUploadID: "up-1"}, nodes)
	require.NoError(t, err)
	assert.True(t, repaired)

	cid := trackCidOf(t, pool, 100)
	require.NotNil(t, cid)
	assert.Equal(t, "QmAgreed", *cid)
	assert.Equal(t, trackCidQuorum, calls, "stop asking nodes once quorum is reached")
}

// track_cid decides which bytes every listener receives, so one node's word is
// never enough to set it.
func TestRepairTrackCidsJob_SingleNodeCannotRepair(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	seedCidlessTrack(t, pool, "up-1")

	calls := 0
	nodes := []string{nodeServing(t, `{"status":"done","results":{"320":"QmLonely"}}`, &calls)}

	job := newTrackCidJob(pool)
	repaired, err := job.repairTrackCid(context.Background(),
		cidlessTrack{TrackID: 100, AudioUploadID: "up-1"}, nodes)
	require.NoError(t, err)
	assert.False(t, repaired)
	assert.Nil(t, trackCidOf(t, pool, 100))
}

// Disagreement means something is wrong upstream. Leave the row alone rather
// than picking a winner.
func TestRepairTrackCidsJob_DisagreementLeavesTrackAlone(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	seedCidlessTrack(t, pool, "up-1")

	calls := 0
	nodes := []string{
		nodeServing(t, `{"status":"done","results":{"320":"QmOne"}}`, &calls),
		nodeServing(t, `{"status":"done","results":{"320":"QmTwo"}}`, &calls),
		nodeServing(t, `{"status":"done","results":{"320":"QmThree"}}`, &calls),
	}

	job := newTrackCidJob(pool)
	repaired, err := job.repairTrackCid(context.Background(),
		cidlessTrack{TrackID: 100, AudioUploadID: "up-1"}, nodes)
	require.NoError(t, err)
	assert.False(t, repaired)
	assert.Nil(t, trackCidOf(t, pool, 100))
}

// Unreachable nodes must not block a repair that the reachable ones agree on.
func TestRepairTrackCidsJob_SkipsUnreachableNodes(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()
	seedCidlessTrack(t, pool, "up-1")

	calls := 0
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer dead.Close()

	body := `{"status":"done","results":{"320":"QmAgreed"}}`
	nodes := []string{
		dead.URL,
		nodeServing(t, body, &calls),
		nodeServing(t, body, &calls),
	}

	job := newTrackCidJob(pool)
	repaired, err := job.repairTrackCid(context.Background(),
		cidlessTrack{TrackID: 100, AudioUploadID: "up-1"}, nodes)
	require.NoError(t, err)
	assert.True(t, repaired)

	cid := trackCidOf(t, pool, 100)
	require.NotNil(t, cid)
	assert.Equal(t, "QmAgreed", *cid)
}

// A track that already has audio is not a candidate, and neither is a deleted
// one - its audio is meant to stay unreachable.
func TestRepairTrackCidsJob_QueryTracksSelectsOnlyRepairable(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01", "handle": "testuser1"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "Repairable", "audio_upload_id": "up-1"},
			{"track_id": 101, "owner_id": 1, "title": "Has Cid", "audio_upload_id": "up-2", "track_cid": "QmAlready"},
			{"track_id": 102, "owner_id": 1, "title": "Deleted", "audio_upload_id": "up-3", "is_delete": true},
			{"track_id": 103, "owner_id": 1, "title": "No Upload Id"},
		},
	})

	job := newTrackCidJob(pool)
	tracks, err := job.queryTracks(context.Background())
	require.NoError(t, err)

	ids := []int64{}
	for _, tr := range tracks {
		ids = append(ids, tr.TrackID)
	}
	assert.Equal(t, []int64{100}, ids, fmt.Sprintf("got %+v", tracks))
}

// The indexer writing real metadata must always beat the repair job.
func TestApplyTrackCidDoesNotOverwriteExistingCid(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x01", "handle": "testuser1"},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "Has Cid", "track_cid": "QmReal"},
		},
	})

	job := newTrackCidJob(pool)
	require.NoError(t, job.applyTrackCid(context.Background(), 100, "QmRepaired"))

	cid := trackCidOf(t, pool, 100)
	require.NotNil(t, cid)
	assert.Equal(t, "QmReal", *cid)
}
