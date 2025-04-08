package dbv1

import (
	"context"
	"encoding/json"
	"fmt"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
)

type FullTracksParams GetTracksParams

type FullTrack struct {
	GetTracksRow

	Artwork SquareImage `json:"artwork"`
	UserID  string      `json:"user_id"`
	User    FullUser    `json:"user"`

	FolloweeReposts   []*FolloweeRepost   `json:"followee_reposts"`
	FolloweeFavorites []*FolloweeFavorite `json:"followee_favorites"`
}

func (q *Queries) FullTracks(ctx context.Context, arg GetTracksParams) ([]FullTrack, error) {
	rawTracks, err := q.GetTracks(ctx, GetTracksParams(arg))
	if err != nil {
		return nil, err
	}

	userIds := []int32{}
	for _, track := range rawTracks {
		userIds = append(userIds, track.UserID)
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

	trackMap := map[int32]FullTrack{}
	for _, track := range rawTracks {
		track.ID, _ = trashid.EncodeHashId(int(track.TrackID))
		user, ok := userMap[track.UserID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}

		// re-encode ids for followee_favorites + followee_reposts
		var followeeReposts []*FolloweeRepost
		if err = json.Unmarshal(track.FolloweeReposts, &followeeReposts); err == nil {
			for _, r := range followeeReposts {
				r.RepostItemId = trashid.StringEncode(r.RepostItemId)
				r.UserId = trashid.StringEncode(r.UserId)
			}
		}

		var followeeFavorites []*FolloweeFavorite
		if err = json.Unmarshal(track.FolloweeFavorites, &followeeFavorites); err == nil {
			for _, r := range followeeFavorites {
				r.FavoriteItemId = trashid.StringEncode(r.FavoriteItemId)
				r.UserId = trashid.StringEncode(r.UserId)
			}
		}

		fullTrack := FullTrack{
			GetTracksRow:      track,
			Artwork:           squareImageStruct(track.CoverArtSizes, track.CoverArt),
			User:              user,
			UserID:            user.ID,
			FolloweeFavorites: followeeFavorites,
			FolloweeReposts:   followeeReposts,
		}
		trackMap[track.TrackID] = fullTrack
	}

	// return in same order as input list of ids
	// some ids may be not found...
	fullTracks := []FullTrack{}
	for _, id := range arg.Ids {
		if t, found := trackMap[id]; found {
			fullTracks = append(fullTracks, t)
		}
	}

	return fullTracks, nil
}

type MinTrack struct {
	ID                       string        `json:"id"`
	Title                    pgtype.Text   `json:"title"`
	User                     MinUser       `json:"user"`
	Artwork                  SquareImage   `json:"artwork"`
	Duration                 pgtype.Int4   `json:"duration"`
	Description              pgtype.Text   `json:"description"`
	Genre                    pgtype.Text   `json:"genre"`
	TrackCid                 pgtype.Text   `json:"track_cid"`
	PreviewCid               pgtype.Text   `json:"preview_cid"`
	OrigFileCid              pgtype.Text   `json:"orig_file_cid"`
	OrigFilename             pgtype.Text   `json:"orig_filename"`
	IsOriginalAvailable      bool          `json:"is_original_available"`
	Mood                     pgtype.Text   `json:"mood"`
	ReleaseDate              interface{}   `json:"release_date"`
	RemixOf                  interface{}   `json:"remix_of"`
	RepostCount              int32         `json:"repost_count"`
	FavoriteCount            int32         `json:"favorite_count"`
	CommentCount             pgtype.Int4   `json:"comment_count"`
	Tags                     pgtype.Text   `json:"tags"`
	IsDownloadable           bool          `json:"is_downloadable"`
	PlayCount                pgtype.Int8   `json:"play_count"`
	PinnedCommentID          pgtype.Int4   `json:"pinned_comment_id"`
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
		Permalink:                fmt.Sprintf("/%s/%s", fullTrack.User.Handle.String, fullTrack.Slug.String),
	}
}

func ToMinTracks(fullTracks []FullTrack) []MinTrack {
	result := make([]MinTrack, len(fullTracks))
	for i, track := range fullTracks {
		result[i] = ToMinTrack(track)
	}
	return result
}
