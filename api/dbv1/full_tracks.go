package dbv1

import (
	"context"
	"encoding/json"
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
	Track FullTrack
}

func (m MinTrack) MarshalJSON() ([]byte, error) {
	result := map[string]interface{}{
		"id":                         m.Track.ID,
		"title":                      m.Track.Title,
		"user":                       ToMinUser(m.Track.User),
		"artwork":                    m.Track.Artwork,
		"duration":                   m.Track.Duration,
		"description":                m.Track.Description,
		"genre":                      m.Track.Genre,
		"track_cid":                  m.Track.TrackCid,
		"preview_cid":                m.Track.PreviewCid,
		"orig_file_cid":              m.Track.OrigFileCid,
		"orig_filename":              m.Track.OrigFilename,
		"is_original_available":      m.Track.IsOriginalAvailable,
		"mood":                       m.Track.Mood,
		"release_date":               m.Track.ReleaseDate,
		"remix_of":                   m.Track.RemixOf,
		"repost_count":               m.Track.RepostCount,
		"favorite_count":             m.Track.FavoriteCount,
		"comment_count":              m.Track.CommentCount,
		"tags":                       m.Track.Tags,
		"is_downloadable":            m.Track.IsDownloadable,
		"play_count":                 m.Track.PlayCount,
		"pinned_comment_id":          m.Track.PinnedCommentID,
		"playlists_containing_track": []interface{}{}, // TODO
		"album_backlink":             nil,
		"is_streamable":              !m.Track.IsDelete && !m.Track.User.IsDeactivated,
		"permalink":                  fmt.Sprintf("/%s/%s", utils.String(m.Track.User.Handle), utils.String(m.Track.Slug)),
	}

	for key, value := range result {
		if value == nil {
			delete(result, key)
		}
	}

	return json.Marshal(result)
}

func ToMinTrack(fullTrack FullTrack) MinTrack {
	return MinTrack{
		Track: fullTrack,
	}
}
