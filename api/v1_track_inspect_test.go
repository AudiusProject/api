package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/rendezvous"
	"api.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestInspectTrackWithoutHosts(t *testing.T) {
	rendezvous.GlobalHasher = rendezvous.NewRendezvousHasher([]string{})

	track := dbv1.Track{
		GetTracksRow: dbv1.GetTracksRow{
			TrackCid: pgtype.Text{String: "track-cid", Valid: true},
		},
	}

	info, err := inspectTrack(track, false)
	assert.Nil(t, info)
	assert.ErrorContains(t, err, "failed to fetch blob info from any host")
}

func TestV1TrackInspectTrackNotFound(t *testing.T) {
	app := emptyTestApp(t)

	trackID := trashid.MustEncodeHashID(999999)
	status, body := testGet(t, app, "/v1/tracks/"+trackID+"/inspect")
	assert.Equal(t, 404, status)
	jsonAssert(t, body, map[string]any{
		"error": "track not found",
	})
}

func TestV1TracksInspectNoTracksFound(t *testing.T) {
	app := emptyTestApp(t)

	trackID := trashid.MustEncodeHashID(999998)
	status, body := testGet(t, app, "/v1/tracks/inspect?id="+trackID)
	assert.Equal(t, 404, status)
	jsonAssert(t, body, map[string]any{
		"error": "no tracks found",
	})
}
