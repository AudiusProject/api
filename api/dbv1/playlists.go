package dbv1

import (
	"context"
	"fmt"

	"api.audius.co/trashid"
)

type PlaylistsParams struct {
	GetPlaylistsParams
	OmitTracks bool
	TrackLimit int // 0 means use default (200), positive values set the limit
}

type Playlist struct {
	GetPlaylistsRow

	ID         string         `json:"id"`
	Artwork    *SquareImage   `json:"artwork"`
	UserID     trashid.HashId `json:"user_id"`
	User       User       `json:"user"`
	Tracks     []Track    `json:"tracks"`
	TrackCount int32          `json:"track_count"`
	Access     Access         `json:"access"`
	Permalink  string         `json:"permalink"`

	FolloweeReposts   []*FolloweeRepost          `json:"followee_reposts"`
	FolloweeFavorites []*FolloweeFavorite        `json:"followee_favorites"`
	PlaylistContents  []PlaylistContentsItem `json:"playlist_contents"`
	AddedTimestamps   []PlaylistContentsItem `json:"added_timestamps"`
}

type PlaylistContentsItem struct {
	Time         float64 `json:"timestamp"`
	TrackId      string  `json:"track_id"`
	MetadataTime float64 `json:"metadata_timestamp"`
}

func (q *Queries) PlaylistsKeyed(ctx context.Context, arg PlaylistsParams) (map[int32]Playlist, error) {
	rawPlaylists, err := q.GetPlaylists(ctx, arg.GetPlaylistsParams)
	if err != nil {
		return nil, err
	}

	// pluck user + track IDs
	trackIds := []int32{}
	userIds := make([]int32, len(rawPlaylists))
	for idx, p := range rawPlaylists {
		userIds[idx] = p.PlaylistOwnerID

		if !arg.OmitTracks {
			trackLimit := 200
			if arg.TrackLimit != 0 {
				trackLimit = arg.TrackLimit
			}
			// some playlists have over a thousand tracks which causes slow load times,
			// so we limit the track hydration here to prevent bad experience.
			trackStubs := p.PlaylistContents.TrackIDs
			if len(trackStubs) > trackLimit {
				trackStubs = trackStubs[:trackLimit]
			}
			for _, t := range trackStubs {
				trackIds = append(trackIds, int32(t.Track))
			}
		}
	}

	// fetch users + tracks in parallel
	loaded, err := q.Parallel(ctx, ParallelParams{
		UserIds:  userIds,
		TrackIds: trackIds,
		MyID:     arg.MyID.(int32),
	})
	if err != nil {
		return nil, err
	}

	playlistMap := map[int32]Playlist{}
	for _, playlist := range rawPlaylists {
		id, _ := trashid.EncodeHashId(int(playlist.PlaylistID))
		user, ok := loaded.UserMap[playlist.PlaylistOwnerID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}

		var tracks = make([]Track, 0, len(playlist.PlaylistContents.TrackIDs))
		for _, t := range playlist.PlaylistContents.TrackIDs {
			if track, ok := loaded.TrackMap[int32(t.Track)]; ok {
				tracks = append(tracks, track)
			}
		}

		// slightly change playlist_contents
		playlistContents := []PlaylistContentsItem{}
		for _, item := range playlist.PlaylistContents.TrackIDs {
			trackId, _ := trashid.EncodeHashId(int(item.Track))
			playlistContents = append(playlistContents, PlaylistContentsItem{
				Time:         item.Time,
				MetadataTime: item.MetadataTime,
				TrackId:      trackId,
			})
		}

		// For playlists, download access is the same as stream access
		streamAccess := q.GetPlaylistAccess(
			ctx,
			arg.MyID.(int32),
			playlist.StreamConditions,
			&playlist,
			&user)
		downloadAccess := streamAccess

		var playlistType string
		if playlist.IsAlbum {
			playlistType = "album"
		} else {
			playlistType = "playlist"
		}

		playlistMap[playlist.PlaylistID] = Playlist{
			GetPlaylistsRow:   playlist,
			ID:                id,
			Artwork:           squareImageStruct(playlist.Artwork),
			User:              user,
			UserID:            user.ID,
			Tracks:            tracks,
			TrackCount:        int32(len(playlist.PlaylistContents.TrackIDs)),
			FolloweeFavorites: fullFolloweeFavorites(playlist.FolloweeFavorites),
			FolloweeReposts:   fullFolloweeReposts(playlist.FolloweeReposts),
			PlaylistContents:  playlistContents,
			Permalink:         fmt.Sprintf("/%s/%s/%s", user.Handle.String, playlistType, playlist.Slug.String),
			AddedTimestamps:   playlistContents,
			Access: Access{
				Stream:   streamAccess,
				Download: downloadAccess,
			},
		}
	}

	return playlistMap, nil
}

func (q *Queries) Playlists(ctx context.Context, arg PlaylistsParams) ([]Playlist, error) {
	playlistMap, err := q.PlaylistsKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	list := make([]Playlist, 0, len(arg.Ids))
	for _, id := range arg.Ids {
		if p, found := playlistMap[id]; found {
			list = append(list, p)
		}
	}

	return list, nil
}
