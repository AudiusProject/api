package dbv1

import (
	"context"
	"fmt"
	"strings"

	"bridgerton.audius.co/rendezvous"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
)

type FullUser struct {
	GetUsersRow

	ArtistPickTrackID *string         `json:"artist_pick_track_id"`
	ProfilePicture    *SquareImage    `json:"profile_picture"`
	CoverPhoto        *RectangleImage `json:"cover_photo"`
}

func (q *Queries) FullUsersKeyed(ctx context.Context, arg GetUsersParams) (map[int32]FullUser, error) {
	rawUsers, err := q.GetUsers(ctx, arg)
	if err != nil {
		return nil, err
	}

	userMap := map[int32]FullUser{}
	for _, user := range rawUsers {
		var coverPhoto *RectangleImage
		{
			cid := ""
			if user.CoverPhotoSizes.Valid {
				cid = user.CoverPhotoSizes.String
			} else if user.CoverPhoto.Valid && !strings.HasPrefix(user.CoverPhoto.String, "{") {
				cid = user.CoverPhoto.String
			}

			if cid != "" {
				// rendezvous for cid
				first, rest := rendezvous.GlobalHasher.ReplicaSet3(cid)

				coverPhoto = &RectangleImage{
					X640:    fmt.Sprintf("%s/content/%s/640x.jpg", first, cid),
					X2000:   fmt.Sprintf("%s/content/%s/2000x.jpg", first, cid),
					Mirrors: rest,
				}
			}
		}

		profilePicture := squareImageStruct(user.ProfilePictureSizes, user.ProfilePicture)

		var artistPickTrackID *string
		if user.ArtistPickTrackID.Valid {
			id, _ := trashid.EncodeHashId(int(user.ArtistPickTrackID.Int32))
			artistPickTrackID = &id
		}

		userMap[user.UserID] = FullUser{
			GetUsersRow:       user,
			ArtistPickTrackID: artistPickTrackID,
			CoverPhoto:        coverPhoto,
			ProfilePicture:    profilePicture,
		}
	}

	return userMap, nil
}

func (q *Queries) FullUsers(ctx context.Context, arg GetUsersParams) ([]FullUser, error) {
	userMap, err := q.FullUsersKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	fullUsers := []FullUser{}

	// return in same order as input list of ids
	for _, id := range arg.Ids {
		if u, found := userMap[id]; found {
			fullUsers = append(fullUsers, u)
		}
	}

	return fullUsers, nil
}

func squareImageStruct(maybeCids ...pgtype.Text) *SquareImage {
	cid := ""
	for _, m := range maybeCids {
		if m.Valid && !strings.HasPrefix(m.String, "{") {
			cid = m.String
			break
		}
	}

	if cid == "" {
		return nil
	}

	// rendezvous for cid
	first, rest := rendezvous.GlobalHasher.ReplicaSet3(cid)

	return &SquareImage{
		X150x150:   fmt.Sprintf("%s/content/%s/150x150.jpg", first, cid),
		X480x480:   fmt.Sprintf("%s/content/%s/480x480.jpg", first, cid),
		X1000x1000: fmt.Sprintf("%s/content/%s/1000x1000.jpg", first, cid),
		Mirrors:    rest,
	}
}

type MinUser struct {
	ID                    trashid.HashId  `json:"id"`
	AlbumCount            pgtype.Int8     `json:"album_count"`
	ArtistPickTrackID     *string         `json:"artist_pick_track_id"`
	Bio                   pgtype.Text     `json:"bio"`
	CoverPhoto            *RectangleImage `json:"cover_photo"`
	FolloweeCount         pgtype.Int8     `json:"followee_count"`
	FollowerCount         pgtype.Int8     `json:"follower_count"`
	Handle                pgtype.Text     `json:"handle"`
	IsVerified            bool            `json:"is_verified"`
	TwitterHandle         pgtype.Text     `json:"twitter_handle"`
	InstagramHandle       pgtype.Text     `json:"instagram_handle"`
	TiktokHandle          pgtype.Text     `json:"tiktok_handle"`
	VerifiedWithTwitter   pgtype.Bool     `json:"verified_with_twitter"`
	VerifiedWithInstagram pgtype.Bool     `json:"verified_with_instagram"`
	VerifiedWithTiktok    pgtype.Bool     `json:"verified_with_tiktok"`
	Website               pgtype.Text     `json:"website"`
	Donation              pgtype.Text     `json:"donation"`
	Location              pgtype.Text     `json:"location"`
	Name                  pgtype.Text     `json:"name"`
	PlaylistCount         pgtype.Int8     `json:"playlist_count"`
	ProfilePicture        *SquareImage    `json:"profile_picture"`
	RepostCount           pgtype.Int8     `json:"repost_count"`
	TrackCount            int64           `json:"track_count"`
	IsDeactivated         bool            `json:"is_deactivated"`
	IsAvailable           bool            `json:"is_available"`
	ErcWallet             pgtype.Text     `json:"erc_wallet"`
	SplWallet             pgtype.Text     `json:"spl_wallet"`
	SplUsdcWallet         pgtype.Text     `json:"spl_usdc_wallet"`
	SplUsdcPayoutWallet   pgtype.Text     `json:"spl_usdc_payout_wallet"`
	SupporterCount        int32           `json:"supporter_count"`
	SupportingCount       int32           `json:"supporting_count"`
	TotalAudioBalance     int32           `json:"total_audio_balance"`
	Wallet                pgtype.Text     `json:"wallet"`
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
		SplUsdcWallet:         fullUser.UsdcWallet,
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
