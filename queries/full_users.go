package queries

import (
	"context"

	"bridgerton.audius.co/trashid"
)

type FullUser struct {
	GetUsersRow
}

func (q *Queries) FullUsers(ctx context.Context, arg GetUsersParams) ([]FullUser, error) {
	rawUsers, err := q.GetUsers(ctx, arg)
	if err != nil {
		return nil, err
	}

	fullUsers := make([]FullUser, len(rawUsers))
	for idx, user := range rawUsers {

		// playlist_library only populated for current user
		if user.UserID != arg.MyID {
			user.PlaylistLibrary = []byte("null")
		}

		user.ID, _ = trashid.EncodeHashId(int(user.UserID))

		fullUsers[idx] = FullUser{
			user,
		}
	}

	return fullUsers, nil
}
