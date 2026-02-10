package dbv1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
	"api.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
)

// User is the standard user type containing all user data
type User struct {
	GetUsersRow

	ArtistPickTrackID *string         `json:"artist_pick_track_id"`
	ProfilePicture    *SquareImage    `json:"profile_picture"`
	CoverPhoto        *RectangleImage `json:"cover_photo"`
	// SplUsdcWallet uses v1 naming convention (preferred over UsdcWallet from GetUsersRow)
	SplUsdcWallet pgtype.Text `json:"spl_usdc_wallet"`
}

// MarshalJSON implements custom JSON marshaling to ensure v1 field naming
// (e.g., spl_usdc_wallet instead of usdc_wallet)
func (u User) MarshalJSON() ([]byte, error) {
	// Use a type alias to avoid infinite recursion
	type Alias User
	userBytes, err := json.Marshal((*Alias)(&u))
	if err != nil {
		return nil, err
	}

	var userMap map[string]interface{}
	if err := json.Unmarshal(userBytes, &userMap); err != nil {
		return nil, err
	}

	// Remove usdc_wallet and ensure spl_usdc_wallet is present (v1 naming)
	delete(userMap, "usdc_wallet")
	userMap["spl_usdc_wallet"] = u.SplUsdcWallet

	return json.Marshal(userMap)
}

func (q *Queries) UsersKeyed(ctx context.Context, arg GetUsersParams) (map[int32]User, error) {
	rawUsers, err := q.GetUsers(ctx, arg)
	if err != nil {
		return nil, err
	}

	userMap := map[int32]User{}
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
				rest = append(rest, config.Cfg.StoreAllNodes...)
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

		userMap[user.UserID] = User{
			GetUsersRow:       user,
			ArtistPickTrackID: artistPickTrackID,
			CoverPhoto:        coverPhoto,
			ProfilePicture:    profilePicture,
			SplUsdcWallet:     user.UsdcWallet, // Map UsdcWallet to SplUsdcWallet for v1 naming
		}
	}

	return userMap, nil
}

func (q *Queries) Users(ctx context.Context, arg GetUsersParams) ([]User, error) {
	userMap, err := q.UsersKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	users := []User{}

	// return in same order as input list of ids
	for _, id := range arg.Ids {
		if u, found := userMap[id]; found {
			users = append(users, u)
		}
	}

	return users, nil
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
	rest = append(rest, config.Cfg.StoreAllNodes...)

	return &SquareImage{
		X150x150:   fmt.Sprintf("%s/content/%s/150x150.jpg", first, cid),
		X480x480:   fmt.Sprintf("%s/content/%s/480x480.jpg", first, cid),
		X1000x1000: fmt.Sprintf("%s/content/%s/1000x1000.jpg", first, cid),
		Mirrors:    rest,
	}
}

