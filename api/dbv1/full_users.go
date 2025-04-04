package dbv1

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
	ID                    string         `json:"id"`
	AlbumCount            *int64         `json:"album_count"`
	ArtistPickTrackID     *string        `json:"artist_pick_track_id"`
	Bio                   *string        `json:"bio"`
	CoverPhoto            RectangleImage `json:"cover_photo"`
	FolloweeCount         *int64         `json:"followee_count"`
	FollowerCount         *int64         `json:"follower_count"`
	Handle                *string        `json:"handle"`
	IsVerified            bool           `json:"is_verified"`
	TwitterHandle         *string        `json:"twitter_handle"`
	InstagramHandle       *string        `json:"instagram_handle"`
	TiktokHandle          *string        `json:"tiktok_handle"`
	VerifiedWithTwitter   *bool          `json:"verified_with_twitter"`
	VerifiedWithInstagram *bool          `json:"verified_with_instagram"`
	VerifiedWithTiktok    *bool          `json:"verified_with_tiktok"`
	Website               *string        `json:"website"`
	Donation              *string        `json:"donation"`
	Location              *string        `json:"location"`
	Name                  *string        `json:"name"`
	PlaylistCount         *int64         `json:"playlist_count"`
	ProfilePicture        SquareImage    `json:"profile_picture"`
	RepostCount           *int64         `json:"repost_count"`
	TrackCount            *int64         `json:"track_count"`
	IsDeactivated         bool           `json:"is_deactivated"`
	IsAvailable           bool           `json:"is_available"`
	ErcWallet             *string        `json:"erc_wallet"`
	SplWallet             *string        `json:"spl_wallet"`
	SplUsdcPayoutWallet   *string        `json:"spl_usdc_payout_wallet"`
	SupporterCount        int32          `json:"supporter_count"`
	SupportingCount       int32          `json:"supporting_count"`
	TotalAudioBalance     int32          `json:"total_audio_balance"`
	Wallet                *string        `json:"wallet"`
}

func ToMinUser(fullUser FullUser) MinUser {
	return MinUser{
		ID:                    fullUser.ID,
		AlbumCount:            fullUser.AlbumCount,
		ArtistPickTrackID:     fullUser.ArtistPickTrackID,
		Bio:                   fullUser.Bio,
		CoverPhoto:            fullUser.CoverPhoto,
		FolloweeCount:         fullUser.FolloweeCount,
		FollowerCount:         fullUser.FollowerCount,
		Handle:                fullUser.Handle,
		IsVerified:            fullUser.IsVerified,
		TwitterHandle:         fullUser.TwitterHandle,
		InstagramHandle:       fullUser.InstagramHandle,
		TiktokHandle:          fullUser.TiktokHandle,
		VerifiedWithTwitter:   fullUser.VerifiedWithTwitter,
		VerifiedWithInstagram: fullUser.VerifiedWithInstagram,
		VerifiedWithTiktok:    fullUser.VerifiedWithTiktok,
		Website:               fullUser.Website,
		Donation:              fullUser.Donation,
		Location:              fullUser.Location,
		Name:                  fullUser.Name,
		PlaylistCount:         fullUser.PlaylistCount,
		ProfilePicture:        fullUser.ProfilePicture,
		RepostCount:           fullUser.RepostCount,
		TrackCount:            fullUser.TrackCount,
		IsDeactivated:         fullUser.IsDeactivated,
		IsAvailable:           fullUser.IsAvailable,
		ErcWallet:             fullUser.ErcWallet,
		SplWallet:             fullUser.SplWallet,
		SplUsdcPayoutWallet:   fullUser.SplUsdcPayoutWallet,
		SupporterCount:        fullUser.SupporterCount,
		SupportingCount:       fullUser.SupportingCount,
		TotalAudioBalance:     fullUser.TotalAudioBalance,
		Wallet:                fullUser.Wallet,
	}
}

func ToMinUsers(fullUsers []FullUser) []MinUser {
	result := make([]MinUser, len(fullUsers))
	for i, user := range fullUsers {
		result[i] = ToMinUser(user)
	}
	return result
}
