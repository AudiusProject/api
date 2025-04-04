package dbv1

import (
	"context"
	"fmt"

	"bridgerton.audius.co/trashid"
	"bridgerton.audius.co/utils"
)

type FullTracksParams GetTracksParams

type FullTrack struct {
	GetTracksRow

	Artwork SquareImage `json:"artwork"`
	UserID  string      `json:"user_id"`
	User    FullUser    `json:"user"`
}

func (q *Queries) FullTracks(ctx context.Context, arg GetTracksParams) ([]FullTrack, error) {
	rawTracks, err := q.GetTracks(ctx, GetTracksParams(arg))
	if err != nil {
		return nil, err
	}

	userIds := make([]int32, len(rawTracks))
	for idx, track := range rawTracks {
		userIds[idx] = track.UserID
	}

	users, err := q.FullUsers(ctx, GetUsersParams{
		MyID: arg.MyID,
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	userMap := map[int32]FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	fullTracks := make([]FullTrack, 0, len(rawTracks))
	for _, track := range rawTracks {
		track.ID, _ = trashid.EncodeHashId(int(track.TrackID))
		user, ok := userMap[track.UserID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}
		fullTracks = append(fullTracks, FullTrack{
			GetTracksRow: track,
			Artwork:      squareImageStruct(track.CoverArtSizes, track.CoverArt),
			User:         user,
			UserID:       user.ID,
		})
	}

	return fullTracks, nil
}

type MinTrack struct {
	ID                       string        `json:"id"`
	Title                    *string       `json:"title"`
	User                     MinUser       `json:"user"`
	Artwork                  SquareImage   `json:"artwork"`
	Duration                 *int32        `json:"duration"`
	Description              *string       `json:"description"`
	Genre                    *string       `json:"genre"`
	TrackCid                 *string       `json:"track_cid"`
	PreviewCid               *string       `json:"preview_cid"`
	OrigFileCid              *string       `json:"orig_file_cid"`
	OrigFilename             *string       `json:"orig_filename"`
	IsOriginalAvailable      bool          `json:"is_original_available"`
	Mood                     *string       `json:"mood"`
	ReleaseDate              interface{}   `json:"release_date"`
	RemixOf                  interface{}   `json:"remix_of"`
	RepostCount              int32         `json:"repost_count"`
	FavoriteCount            int32         `json:"favorite_count"`
	CommentCount             *int32        `json:"comment_count"`
	Tags                     *string       `json:"tags"`
	IsDownloadable           bool          `json:"is_downloadable"`
	PlayCount                *int64        `json:"play_count"`
	PinnedCommentID          *int32        `json:"pinned_comment_id"`
	PlaylistsContainingTrack []interface{} `json:"playlists_containing_track"`
	AlbumBacklink            interface{}   `json:"album_backlink"`
	IsStreamable             bool          `json:"is_streamable"`
	Permalink                string        `json:"permalink"`
}

func ToMinTrack(fullTrack FullTrack) MinTrack {
	return MinTrack{
		ID:                       fullTrack.ID,
		Title:                    fullTrack.Title,
		User:                     ToMinUser(fullTrack.User),
		Artwork:                  fullTrack.Artwork,
		Duration:                 fullTrack.Duration,
		Description:              fullTrack.Description,
		Genre:                    fullTrack.Genre,
		TrackCid:                 fullTrack.TrackCid,
		PreviewCid:               fullTrack.PreviewCid,
		OrigFileCid:              fullTrack.OrigFileCid,
		OrigFilename:             fullTrack.OrigFilename,
		IsOriginalAvailable:      fullTrack.IsOriginalAvailable,
		Mood:                     fullTrack.Mood,
		ReleaseDate:              fullTrack.ReleaseDate,
		RemixOf:                  fullTrack.RemixOf,
		RepostCount:              fullTrack.RepostCount,
		FavoriteCount:            fullTrack.FavoriteCount,
		CommentCount:             fullTrack.CommentCount,
		Tags:                     fullTrack.Tags,
		IsDownloadable:           fullTrack.IsDownloadable,
		PlayCount:                fullTrack.PlayCount,
		PinnedCommentID:          fullTrack.PinnedCommentID,
		PlaylistsContainingTrack: []interface{}{}, // TODO
		AlbumBacklink:            nil,
		IsStreamable:             !fullTrack.IsDelete && !fullTrack.User.IsDeactivated,
		Permalink:                fmt.Sprintf("/%s/%s", utils.String(fullTrack.User.Handle), utils.String(fullTrack.Slug)),
	}
}

func ToMinTracks(fullTracks []FullTrack) []MinTrack {
	result := make([]MinTrack, len(fullTracks))
	for i, track := range fullTracks {
		result[i] = ToMinTrack(track)
	}
	return result
}
