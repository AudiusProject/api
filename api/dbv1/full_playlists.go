package dbv1

import (
	"context"
	"fmt"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
)

type FullPlaylistsParams struct {
	GetPlaylistsParams
	OmitTracks bool
	TrackLimit int // 0 means use default (200), positive values set the limit
}

type FullPlaylist struct {
	GetPlaylistsRow

	ID         string         `json:"id"`
	Artwork    *SquareImage   `json:"artwork"`
	UserID     trashid.HashId `json:"user_id"`
	User       FullUser       `json:"user"`
	Tracks     []FullTrack    `json:"tracks"`
	TrackCount int32          `json:"track_count"`
	Access     Access         `json:"access"`
	Permalink  string         `json:"permalink"`

	FolloweeReposts   []*FolloweeRepost          `json:"followee_reposts"`
	FolloweeFavorites []*FolloweeFavorite        `json:"followee_favorites"`
	PlaylistContents  []FullPlaylistContentsItem `json:"playlist_contents"`
	AddedTimestamps   []FullPlaylistContentsItem `json:"added_timestamps"`
}

type FullPlaylistContentsItem struct {
	Time         float64 `json:"timestamp"`
	TrackId      string  `json:"track_id"`
	MetadataTime float64 `json:"metadata_timestamp"`
}

func (q *Queries) FullPlaylistsKeyed(ctx context.Context, arg FullPlaylistsParams) (map[int32]FullPlaylist, error) {
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

	playlistMap := map[int32]FullPlaylist{}
	for _, playlist := range rawPlaylists {
		id, _ := trashid.EncodeHashId(int(playlist.PlaylistID))
		user, ok := loaded.UserMap[playlist.PlaylistOwnerID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}

		var tracks = make([]FullTrack, 0, len(playlist.PlaylistContents.TrackIDs))
		for _, t := range playlist.PlaylistContents.TrackIDs {
			if track, ok := loaded.TrackMap[int32(t.Track)]; ok {
				tracks = append(tracks, track)
			}
		}

		// slightly change playlist_contents
		fullPlaylistContents := []FullPlaylistContentsItem{}
		for _, item := range playlist.PlaylistContents.TrackIDs {
			trackId, _ := trashid.EncodeHashId(int(item.Track))
			fullPlaylistContents = append(fullPlaylistContents, FullPlaylistContentsItem{
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

		playlistMap[playlist.PlaylistID] = FullPlaylist{
			GetPlaylistsRow:   playlist,
			ID:                id,
			Artwork:           squareImageStruct(playlist.Artwork),
			User:              user,
			UserID:            user.ID,
			Tracks:            tracks,
			TrackCount:        int32(len(tracks)),
			FolloweeFavorites: fullFolloweeFavorites(playlist.FolloweeFavorites),
			FolloweeReposts:   fullFolloweeReposts(playlist.FolloweeReposts),
			PlaylistContents:  fullPlaylistContents,
			Permalink:         fmt.Sprintf("/%s/%s/%s", user.Handle.String, playlistType, playlist.Slug.String),
			AddedTimestamps:   fullPlaylistContents,
			Access: Access{
				Stream:   streamAccess,
				Download: downloadAccess,
			},
		}
	}

	return playlistMap, nil
}

func (q *Queries) FullPlaylists(ctx context.Context, arg FullPlaylistsParams) ([]FullPlaylist, error) {
	playlistMap, err := q.FullPlaylistsKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	fullPlaylists := []FullPlaylist{}
	for _, id := range arg.Ids {
		if t, found := playlistMap[id]; found {
			fullPlaylists = append(fullPlaylists, t)
		}
	}

	return fullPlaylists, nil
}

type MinPlaylist struct {
	ID                   string       `json:"id"`
	PlaylistName         pgtype.Text  `json:"playlist_name"`
	Artwork              *SquareImage `json:"artwork"`
	Access               Access       `json:"access"`
	Description          string       `json:"description"`
	IsImageAutogenerated bool         `json:"is_image_autogenerated"`
	Upc                  string       `json:"upc"`
	DdexApp              string       `json:"ddex_app"`
	PlaylistContents     interface{}  `json:"playlist_contents"`
	TrackCount           int32        `json:"track_count"`
	TotalPlayCount       int64        `json:"total_play_count"`
	IsAlbum              bool         `json:"is_album"`
	FavoriteCount        int32        `json:"favorite_count"`
	RepostCount          int32        `json:"repost_count"`
	User                 MinUser      `json:"user"`
	Permalink            string       `json:"permalink"`
}

func ToMinPlaylist(fullPlaylist FullPlaylist) MinPlaylist {
	minTracks := make([]MinTrack, len(fullPlaylist.Tracks))
	for i, track := range fullPlaylist.Tracks {
		minTracks[i] = ToMinTrack(track)
	}

	return MinPlaylist{
		ID:                   fullPlaylist.ID,
		PlaylistName:         fullPlaylist.PlaylistName,
		Artwork:              fullPlaylist.Artwork,
		Access:               fullPlaylist.Access,
		Upc:                  fullPlaylist.Upc.String,
		DdexApp:              fullPlaylist.DdexApp.String,
		PlaylistContents:     fullPlaylist.PlaylistContents,
		Description:          fullPlaylist.Description.String,
		IsImageAutogenerated: fullPlaylist.IsImageAutogenerated,
		IsAlbum:              fullPlaylist.IsAlbum,
		TrackCount:           int32(len(fullPlaylist.Tracks)),
		TotalPlayCount: func() int64 {
			var total int64
			for _, track := range fullPlaylist.Tracks {
				total += track.PlayCount
			}
			return total
		}(),
		FavoriteCount: int32(fullPlaylist.FavoriteCount.Int32),
		RepostCount:   int32(fullPlaylist.RepostCount.Int32),
		User:          ToMinUser(fullPlaylist.User),
		Permalink:     fullPlaylist.Permalink,
	}
}

func ToMinPlaylists(fullPlaylists []FullPlaylist) []MinPlaylist {
	result := make([]MinPlaylist, len(fullPlaylists))
	for i, playlist := range fullPlaylists {
		result[i] = ToMinPlaylist(playlist)
	}
	return result
}
