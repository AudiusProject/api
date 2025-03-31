package queries

import (
	"context"

	"bridgerton.audius.co/trashid"
)

type FullTracksParams GetTracksParams

type FullTrack struct {
	GetTracksRow

	User FullUser `json:"user"`
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
		track.ID, _ = trashid.EncodeHashId(int(track.UserID))
		user, ok := userMap[track.UserID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}
		fullTracks = append(fullTracks, FullTrack{
			GetTracksRow: track,
			User:         user,
		})
	}

	return fullTracks, nil
}
