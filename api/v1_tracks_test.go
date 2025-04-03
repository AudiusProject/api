package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetTracks(t *testing.T) {
	// someone else can only see public tracks
	{
		tracks, err := app.queries.FullTracks(t.Context(), dbv1.GetTracksParams{
			MyID:    1,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 1)
		assert.True(t, tracks[0].HasCurrentUserReposted)
	}

	// I can see all my tracks
	{
		tracks, err := app.queries.FullTracks(t.Context(), dbv1.GetTracksParams{
			MyID:    2,
			OwnerID: 2,
		})
		assert.NoError(t, err)
		assert.Len(t, tracks, 2)
	}

	{
		tracks, err := app.queries.FullTracks(t.Context(), dbv1.GetTracksParams{
			MyID: 2,
			Ids:  []int32{301},
		})
		assert.NoError(t, err)
		track := tracks[0]
		assert.Equal(t, 135.0, track.DownloadConditions.UsdcPurchase.Price)

	}
}
