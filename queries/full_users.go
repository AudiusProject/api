package queries

import (
	"context"
	"fmt"
	"strings"

	"bridgerton.audius.co/rendezvous"
	"bridgerton.audius.co/trashid"
)

type FullUser struct {
	GetUsersRow

	ArtistPickTrackID *string        `json:"artist_pick_track_id"`
	ProfilePicture    SquareImage    `json:"profile_picture"`
	CoverPhoto        RectangleImage `json:"cover_photo"`
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

		// profile picture + cover photo
		var coverPhoto RectangleImage
		{
			cid := ""
			if user.CoverPhotoSizes != nil {
				cid = *user.CoverPhotoSizes
			}
			if cid == "" && user.CoverPhoto != nil && !strings.HasPrefix(*user.CoverPhoto, "{") {
				cid = *user.CoverPhoto
			}

			// rendezvous for cid
			rankedHosts := rendezvous.GlobalHasher.Rank(cid)
			first := rankedHosts[0]
			rest := rankedHosts[1:3]

			coverPhoto = RectangleImage{
				X640:    fmt.Sprintf("%s/content/%s/640x.jpg", first, cid),
				X2000:   fmt.Sprintf("%s/content/%s/2000x.jpg", first, cid),
				Mirrors: rest,
			}
		}

		// profile_picture
		var profilePicture SquareImage
		{
			cid := ""
			if user.ProfilePictureSizes != nil {
				cid = *user.ProfilePictureSizes
			}
			if cid == "" && user.ProfilePicture != nil && !strings.HasPrefix(*user.ProfilePicture, "{") {
				cid = *user.ProfilePicture
			}

			// rendezvous for cid
			rankedHosts := rendezvous.GlobalHasher.Rank(cid)
			first := rankedHosts[0]
			rest := rankedHosts[1:3]

			profilePicture = SquareImage{
				X150x150:   fmt.Sprintf("%s/content/%s/150x150.jpg", first, cid),
				X480x480:   fmt.Sprintf("%s/content/%s/480x480.jpg", first, cid),
				X1000x1000: fmt.Sprintf("%s/content/%s/1000x1000.jpg", first, cid),
				Mirrors:    rest,
			}
		}

		var artistPickTrackID *string
		if user.ArtistPickTrackID != nil {
			id, _ := trashid.EncodeHashId(int(*user.ArtistPickTrackID))
			artistPickTrackID = &id
		}

		fullUsers[idx] = FullUser{
			GetUsersRow:       user,
			ArtistPickTrackID: artistPickTrackID,
			CoverPhoto:        coverPhoto,
			ProfilePicture:    profilePicture,
		}
	}

	return fullUsers, nil
}
