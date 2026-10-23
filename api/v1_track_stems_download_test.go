package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/api/testdata"
	"api.audius.co/database"
	"api.audius.co/rendezvous"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeContentNode serves blob info and cidstream for the given blobs, with
// Range support like a real node.
func fakeContentNode(t *testing.T, blobs map[string][]byte) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cid, ok := strings.CutPrefix(r.URL.Path, "/internal/blobs/info/"); ok {
			data, found := blobs[cid]
			if !found {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `{"ContentType":"audio/wav","Size":%d}`, len(data))
			return
		}
		if cid, ok := strings.CutPrefix(r.URL.Path, "/tracks/cidstream/"); ok {
			data, found := blobs[cid]
			if !found {
				http.NotFound(w, r)
				return
			}
			http.ServeContent(w, r, cid, time.Time{}, bytes.NewReader(data))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func useContentNodes(t *testing.T, hosts ...string) {
	prev := rendezvous.GlobalHasher
	rendezvous.GlobalHasher = rendezvous.NewRendezvousHasher(hosts)
	t.Cleanup(func() { rendezvous.GlobalHasher = prev })
}

func blob(size int, seed byte) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i) ^ seed
	}
	return b
}

func readZip(t *testing.T, body []byte) map[string][]byte {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err, f.Name)
		rc.Close()
		files[f.Name] = data
	}
	return files
}

func TestStemsZipLengthMatchesOutput(t *testing.T) {
	blobs := map[string][]byte{
		"QmA": blob(200_000, 1),
		"QmB": blob(1, 2),
		"QmC": blob(70_000, 3),
	}
	srv := fakeContentNode(t, blobs)

	entries := []stemsZipEntry{
		{name: "🍕DRUMS.wav", size: 200_000, link: &dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/QmA"}},
		{name: "bass.wav", size: 1, link: &dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/QmB"}},
		{name: "vocals.wav", size: 70_000, link: &dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/QmC"}},
	}
	headers := stemsZipHeaders("Alone", entries, time.Now())
	length, err := stemsZipLength(headers)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, writeStemsZip(&buf, headers, entries))
	assert.Equal(t, length, int64(buf.Len()))

	files := readZip(t, buf.Bytes())
	assert.Equal(t, blobs["QmA"], files["Alone/🍕DRUMS.wav"])
	assert.Equal(t, blobs["QmB"], files["Alone/bass.wav"])
	assert.Equal(t, blobs["QmC"], files["Alone/vocals.wav"])
}

func TestStemsZipResumesOnMirror(t *testing.T) {
	data := blob(500_000, 7)

	// The primary sends part of the file, then drops the connection.
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data[:123_456])
		w.(http.Flusher).Flush()
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	t.Cleanup(primary.Close)
	mirror := fakeContentNode(t, map[string][]byte{"QmA": data})

	entries := []stemsZipEntry{{
		name: "stem.wav",
		size: int64(len(data)),
		link: &dbv1.MediaLink{Url: primary.URL + "/tracks/cidstream/QmA", Mirrors: []string{mirror.URL}},
	}}
	headers := stemsZipHeaders("Song", entries, time.Now())
	length, err := stemsZipLength(headers)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, writeStemsZip(&buf, headers, entries))
	assert.Equal(t, length, int64(buf.Len()))
	assert.Equal(t, data, readZip(t, buf.Bytes())["Song/stem.wav"])
}

func TestStemsZipFailsWhenNoMirrorHasFile(t *testing.T) {
	srv := fakeContentNode(t, map[string][]byte{})
	entries := []stemsZipEntry{{
		name: "stem.wav",
		size: 10,
		link: &dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/QmMissing"},
	}}
	err := writeStemsZip(io.Discard, stemsZipHeaders("Song", entries, time.Now()), entries)
	assert.ErrorContains(t, err, "all mirrors failed")
}

func TestStemsZipLengthPastZip64Offsets(t *testing.T) {
	// Two entries that each fit in 32 bits but together push the second
	// header's offset past 4GiB, which changes the central directory.
	entries := []stemsZipEntry{
		{name: "a.wav", size: 3 << 30},
		{name: "b.wav", size: 2 << 30},
	}
	length, err := stemsZipLength(stemsZipHeaders("Big", entries, time.Now()))
	require.NoError(t, err)
	assert.Greater(t, length, int64(5<<30))
}

func TestDedupeZipName(t *testing.T) {
	seen := map[string]int{}
	assert.Equal(t, "Kick.wav", dedupeZipName("Kick.wav", seen))
	assert.Equal(t, "kick (1).wav", dedupeZipName("kick.wav", seen))
	assert.Equal(t, "Kick (2).wav", dedupeZipName("Kick.wav", seen))
}

func TestStemsZipFilename(t *testing.T) {
	track := func(fields map[string]string) *dbv1.Track {
		var tr dbv1.Track
		tr.Title.String = fields["title"]
		tr.OrigFileCid.String = fields["orig_cid"]
		tr.OrigFilename.String = fields["orig_filename"]
		return &tr
	}
	assert.Equal(t, "🍕DRUMS.wav", stemsZipFilename(track(map[string]string{"orig_cid": "Qm", "orig_filename": "  🍕DRUMS.wav"}), "audio/wav"))
	assert.Equal(t, "Bass.wav", stemsZipFilename(track(map[string]string{"title": "Bass", "orig_cid": "Qm"}), "audio/x-wav"))
	assert.Equal(t, "Bass.mp3", stemsZipFilename(track(map[string]string{"title": "Bass"}), "audio/wav"))
	assert.Equal(t, "a_b.mp3", stemsZipFilename(track(map[string]string{"title": "a/b"}), ""))
}

func seedStemsTrack(t *testing.T) *ApiServer {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "artist", "wallet": ownerWallet},
			{"user_id": 2, "handle": "stranger", "wallet": strangerWallet},
		},
		"tracks": {
			{
				"track_id":        1,
				"owner_id":        1,
				"title":           "Alone",
				"orig_file_cid":   "QmParent",
				"orig_filename":   "Alone.wav",
				"is_downloadable": true,
			},
			{
				"track_id":        2,
				"owner_id":        1,
				"title":           "Drums",
				"orig_file_cid":   "QmDrums",
				"orig_filename":   "  Drums.wav",
				"is_downloadable": true,
				"stem_of":         map[string]any{"parent_track_id": 1, "category": "drums"},
			},
			{
				"track_id":        3,
				"owner_id":        1,
				"title":           "Drums again",
				"orig_file_cid":   "QmDrums2",
				"orig_filename":   "drums.wav",
				"is_downloadable": true,
				"stem_of":         map[string]any{"parent_track_id": 1, "category": "drums"},
			},
			{
				"track_id":        4,
				"owner_id":        1,
				"title":           "Vocals",
				"orig_file_cid":   "QmVocals",
				"is_downloadable": false,
				"stem_of":         map[string]any{"parent_track_id": 1, "category": "vocals"},
			},
			{
				"track_id":        5,
				"owner_id":        1,
				"title":           "No Stems",
				"orig_file_cid":   "QmLonely",
				"is_downloadable": true,
			},
		},
		"stems": {
			{"child_track_id": 2, "parent_track_id": 1},
			{"child_track_id": 3, "parent_track_id": 1},
			{"child_track_id": 4, "parent_track_id": 1},
		},
	})
	return app
}

func TestGetTrackStemsDownload(t *testing.T) {
	blobs := map[string][]byte{
		"QmParent": blob(90_000, 1),
		"QmDrums":  blob(40_000, 2),
		"QmDrums2": blob(30_000, 3),
		"QmVocals": blob(20_000, 4),
	}
	srv := fakeContentNode(t, blobs)
	app := seedStemsTrack(t)
	useContentNodes(t, srv.URL)

	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stems/download?include_parent=true", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, 200, res.StatusCode, string(body))
	assert.Equal(t, "application/zip", res.Header.Get("Content-Type"))
	assert.Equal(t, fmt.Sprint(len(body)), res.Header.Get("Content-Length"))
	assert.Contains(t, res.Header.Get("Content-Disposition"), `filename="Alone.zip"`)

	// The vocals stem isn't downloadable, so it's left out.
	assert.Equal(t, map[string][]byte{
		"Alone/Alone.wav":     blobs["QmParent"],
		"Alone/Drums.wav":     blobs["QmDrums"],
		"Alone/drums (1).wav": blobs["QmDrums2"],
	}, readZip(t, body))
}

func TestGetTrackStemsDownload_WithoutParent(t *testing.T) {
	srv := fakeContentNode(t, map[string][]byte{
		"QmParent": blob(10, 1),
		"QmDrums":  blob(10, 2),
		"QmDrums2": blob(10, 3),
	})
	app := seedStemsTrack(t)
	useContentNodes(t, srv.URL)

	status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stems/download")
	require.Equal(t, 200, status, string(body))
	files := readZip(t, body)
	assert.Len(t, files, 2)
	assert.NotContains(t, files, "Alone/Alone.wav")
}

func TestGetTrackStemsDownload_SkipsMissingBlob(t *testing.T) {
	srv := fakeContentNode(t, map[string][]byte{"QmDrums": blob(10, 2)})
	app := seedStemsTrack(t)
	useContentNodes(t, srv.URL)

	status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stems/download?include_parent=true")
	require.Equal(t, 200, status, string(body))
	assert.Equal(t, []string{"Alone/Drums.wav"}, keys(readZip(t, body)))
}

func TestGetTrackStemsDownload_OwnerGetsNonDownloadableStem(t *testing.T) {
	blobs := map[string][]byte{
		"QmDrums":  blob(10, 2),
		"QmDrums2": blob(10, 3),
		"QmVocals": blob(10, 4),
	}
	srv := fakeContentNode(t, blobs)
	app := seedStemsTrack(t)
	useContentNodes(t, srv.URL)

	get := func(wallet string) []string {
		req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stems/download", nil)
		sigData := testdata.GetSignatureData(wallet)
		req.Header.Set("Encoded-Data-Message", sigData.Message)
		req.Header.Set("Encoded-Data-Signature", sigData.Signature)
		res, err := app.Test(req, -1)
		require.NoError(t, err)
		body, _ := io.ReadAll(res.Body)
		require.Equal(t, 200, res.StatusCode, string(body))
		return keys(readZip(t, body))
	}

	assert.Contains(t, get(ownerWallet), "Alone/Vocals.wav")
	assert.NotContains(t, get(strangerWallet), "Alone/Vocals.wav")
}

func TestGetTrackStemsDownload_NotFound(t *testing.T) {
	srv := fakeContentNode(t, map[string][]byte{})
	app := seedStemsTrack(t)
	useContentNodes(t, srv.URL)

	status, _ := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(5)+"/stems/download")
	assert.Equal(t, 404, status)

	status, _ = testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(999)+"/stems/download")
	assert.Equal(t, 404, status)

	// Stems exist but no blob can be found for any of them.
	status, _ = testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stems/download")
	assert.Equal(t, 404, status)
}

func TestDownloadSlots(t *testing.T) {
	s := newDownloadSlots(3, 2)
	assert.True(t, s.acquire("a"))
	assert.True(t, s.acquire("a"))
	assert.False(t, s.acquire("a"))
	assert.True(t, s.acquire("b"))
	assert.False(t, s.acquire("c"))
	s.release("a")
	assert.True(t, s.acquire("c"))
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
