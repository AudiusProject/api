package dbv1

import (
	"context"
	"encoding/json"
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
		var profilePicture = squareImageStruct(user.ProfilePictureSizes, user.ProfilePicture)

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

func squareImageStruct(maybeCids ...*string) SquareImage {
	cid := ""
	for _, m := range maybeCids {
		if m != nil && !strings.HasPrefix(*m, "{") {
			cid = *m
			break
		}
	}

	if cid == "" {
		// todo: what to do here?
		return SquareImage{}
	}

	// rendezvous for cid
	rankedHosts := rendezvous.GlobalHasher.Rank(cid)
	first := rankedHosts[0]
	rest := rankedHosts[1:3]

	return SquareImage{
		X150x150:   fmt.Sprintf("%s/content/%s/150x150.jpg", first, cid),
		X480x480:   fmt.Sprintf("%s/content/%s/480x480.jpg", first, cid),
		X1000x1000: fmt.Sprintf("%s/content/%s/1000x1000.jpg", first, cid),
		Mirrors:    rest,
	}
}

type MinUser struct {
	User FullUser
}

func (m MinUser) MarshalJSON() ([]byte, error) {
	result := map[string]interface{}{
		"id":                      m.User.ID,
		"album_count":             m.User.AlbumCount,
		"artist_pick_track_id":    m.User.ArtistPickTrackID,
		"bio":                     m.User.Bio,
		"cover_photo":             m.User.CoverPhoto,
		"followee_count":          m.User.FolloweeCount,
		"follower_count":          m.User.FollowerCount,
		"handle":                  m.User.Handle,
		"is_verified":             m.User.IsVerified,
		"twitter_handle":          m.User.TwitterHandle,
		"instagram_handle":        m.User.InstagramHandle,
		"tiktok_handle":           m.User.TiktokHandle,
		"verified_with_twitter":   m.User.VerifiedWithTwitter,
		"verified_with_instagram": m.User.VerifiedWithInstagram,
		"verified_with_tiktok":    m.User.VerifiedWithTiktok,
		"website":                 m.User.Website,
		"donation":                m.User.Donation,
		"location":                m.User.Location,
		"name":                    m.User.Name,
		"playlist_count":          m.User.PlaylistCount,
		"profile_picture":         m.User.ProfilePicture,
		"repost_count":            m.User.RepostCount,
		"track_count":             m.User.TrackCount,
		"is_deactivated":          m.User.IsDeactivated,
		"is_available":            m.User.IsAvailable,
		"erc_wallet":              m.User.ErcWallet,
		"spl_wallet":              m.User.SplWallet,
		"spl_usdc_payout_wallet":  m.User.SplUsdcPayoutWallet,
		"supporter_count":         m.User.SupporterCount,
		"supporting_count":        m.User.SupportingCount,
		"total_audio_balance":     m.User.TotalAudioBalance,
		"wallet":                  m.User.Wallet,
	}

	for key, value := range result {
		if value == nil {
			delete(result, key)
		}
	}

	return json.Marshal(result)
}

func ToMinUser(fullUser FullUser) MinUser {
	return MinUser{
		User: fullUser,
	}
}
