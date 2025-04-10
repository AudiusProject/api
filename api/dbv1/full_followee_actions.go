package dbv1

import (
	"encoding/json"

	"bridgerton.audius.co/trashid"
)

func fullFolloweeReposts(raw json.RawMessage) []*FolloweeRepost {
	followeeReposts := []*FolloweeRepost{}
	if err := json.Unmarshal(raw, &followeeReposts); err == nil {
		for _, r := range followeeReposts {
			r.RepostItemId = trashid.StringEncode(r.RepostItemId)
			r.UserId = trashid.StringEncode(r.UserId)
		}
	}
	return followeeReposts
}

func fullFolloweeFavorites(raw json.RawMessage) []*FolloweeFavorite {
	followeeFavorites := []*FolloweeFavorite{}
	if err := json.Unmarshal(raw, &followeeFavorites); err == nil {
		for _, r := range followeeFavorites {
			r.FavoriteItemId = trashid.StringEncode(r.FavoriteItemId)
			r.UserId = trashid.StringEncode(r.UserId)
		}
	}
	return followeeFavorites
}
