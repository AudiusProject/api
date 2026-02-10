package dbv1

import (
	"context"
	"encoding/json"
	"fmt"

	"api.audius.co/trashid"
)

type TracksParams struct {
	GetTracksParams
}

// Track is the standard track type containing all track data
type Track struct {
	GetTracksRow

	Permalink    string         `json:"permalink"`
	IsStreamable bool           `json:"is_streamable"`
	Artwork      *SquareImage   `json:"artwork"`
	Stream       *MediaLink     `json:"stream"`
	Download     *MediaLink     `json:"download"`
	Preview      *MediaLink     `json:"preview"`
	UserID       trashid.HashId `json:"user_id"`
	User         User       `json:"user"`
	Access       Access         `json:"access"`

	FolloweeReposts    []*FolloweeRepost   `json:"followee_reposts"`
	FolloweeFavorites  []*FolloweeFavorite `json:"followee_favorites"`
	RemixOf            FullRemixOf         `json:"remix_of"`
	StreamConditions   *AccessGate         `json:"stream_conditions"`
	DownloadConditions *AccessGate         `json:"download_conditions"`
}

func (q *Queries) TracksKeyed(ctx context.Context, arg TracksParams) (map[int32]Track, error) {
	rawTracks, err := q.GetTracks(ctx, GetTracksParams(arg.GetTracksParams))
	if err != nil {
		return nil, err
	}

	userIds := []int32{}
	collectSplitUserIds := func(usage *AccessGate) {
		if usage == nil || usage.UsdcPurchase == nil {
			return
		}
		for _, split := range usage.UsdcPurchase.Splits {
			userIds = append(userIds, split.UserID)
		}
	}

	for _, rawTrack := range rawTracks {
		userIds = append(userIds, rawTrack.UserID)

		var remixOf RemixOf
		json.Unmarshal(rawTrack.RemixOf, &remixOf)
		for _, r := range remixOf.Tracks {
			userIds = append(userIds, r.ParentUserId)
		}

		collectSplitUserIds(rawTrack.StreamConditions)
		collectSplitUserIds(rawTrack.DownloadConditions)
	}

	userMap, err := q.UsersKeyed(ctx, GetUsersParams{
		MyID: arg.MyID.(int32),
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	// Convert rawTracks to pointers
	trackPtrs := make([]*GetTracksRow, len(rawTracks))
	for i := range rawTracks {
		trackPtrs[i] = &rawTracks[i]
	}

	// Convert userMap to pointers
	userPtrMap := make(map[int32]*User)
	for id, user := range userMap {
		userCopy := user // Create a copy to avoid modifying the original
		userPtrMap[id] = &userCopy
	}

	// Get bulk access for all tracks
	accessMap, err := q.GetBulkTrackAccess(ctx, arg.MyID.(int32), trackPtrs, userPtrMap)
	if err != nil {
		return nil, err
	}

	trackMap := map[int32]Track{}
	for _, rawTrack := range rawTracks {
		rawTrack.ID, _ = trashid.EncodeHashId(int(rawTrack.TrackID))
		user, ok := userMap[rawTrack.UserID]
		if !ok {
			continue
		}

		if rawTrack.FieldVisibility == nil || string(rawTrack.FieldVisibility) == "null" {
			rawTrack.FieldVisibility = []byte(`{
			"mood":null,
			"tags":null,
			"genre":null,
			"share":null,
			"play_count":null,
			"remixes":null
			}`)
		}

		var remixOf RemixOf
		var fullRemixOf FullRemixOf
		json.Unmarshal(rawTrack.RemixOf, &remixOf)
		fullRemixOf = FullRemixOf{
			Tracks: make([]FullRemixOfTrack, len(remixOf.Tracks)),
		}
		for idx, r := range remixOf.Tracks {
			trackId, _ := trashid.EncodeHashId(int(r.ParentTrackId))
			fullRemixOf.Tracks[idx] = FullRemixOfTrack{
				HasRemixAuthorReposted: r.HasRemixAuthorReposted,
				HasRemixAuthorSaved:    r.HasRemixAuthorSaved,
				ParentTrackId:          trackId,
				User:                   userMap[r.ParentUserId],
			}
		}

		// Get access from the bulk access map
		access := accessMap[rawTrack.TrackID]

		id3Tags := &Id3Tags{
			Title:  rawTrack.Title.String,
			Artist: user.Name.String,
		}

		var stream *MediaLink
		if access.Stream {
			stream, err = mediaLink(rawTrack.TrackCid.String, rawTrack.TrackID, arg.MyID.(int32), id3Tags)
			if err != nil {
				return nil, err
			}
		}

		var download *MediaLink
		if rawTrack.IsDownloadable && access.Download {
			cid := rawTrack.OrigFileCid.String
			if cid == "" {
				cid = rawTrack.TrackCid.String
			}
			download, err = mediaLink(cid, rawTrack.TrackID, arg.MyID.(int32), nil)
			if err != nil {
				return nil, err
			}
		}

		var preview *MediaLink
		if rawTrack.PreviewCid.String != "" {
			preview, err = mediaLink(rawTrack.PreviewCid.String, rawTrack.TrackID, arg.MyID.(int32), id3Tags)
			if err != nil {
				return nil, err
			}
		}

		track := Track{
			GetTracksRow:       rawTrack,
			IsStreamable:       !rawTrack.IsDelete && !user.IsDeactivated,
			Permalink:          fmt.Sprintf("/%s/%s", user.Handle.String, rawTrack.Slug.String),
			Artwork:            squareImageStruct(rawTrack.CoverArtSizes, rawTrack.CoverArt),
			Stream:             stream,
			Download:           download,
			Preview:            preview,
			User:               user,
			UserID:             user.ID,
			FolloweeFavorites:  fullFolloweeFavorites(rawTrack.FolloweeFavorites),
			FolloweeReposts:    fullFolloweeReposts(rawTrack.FolloweeReposts),
			RemixOf:            fullRemixOf,
			StreamConditions:   rawTrack.StreamConditions,
			DownloadConditions: rawTrack.DownloadConditions,
			Access:             access,
		}
		trackMap[rawTrack.TrackID] = track
	}

	return trackMap, nil
}

func (q *Queries) Tracks(ctx context.Context, arg TracksParams) ([]Track, error) {
	trackMap, err := q.TracksKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	tracks := []Track{}
	for _, id := range arg.Ids {
		if t, found := trackMap[id]; found {
			tracks = append(tracks, t)
		}
	}

	return tracks, nil
}
