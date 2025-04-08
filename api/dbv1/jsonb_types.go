package dbv1

type UsageConditions *struct {
	UsdcPurchase *struct {
		Price  float64 `json:"price"`
		Splits []struct {
			UserID     int32   `json:"user_id"`
			Percentage float64 `json:"percentage"`
		} `json:"splits"`
	} `json:"usdc_purchase,omitempty"`

	FollowUserID *int64 `json:"follow_user_id,omitempty"`
}

type RectangleImage struct {
	X640    string   `json:"640x"`
	X2000   string   `json:"2000x"`
	Mirrors []string `json:"mirrors"`
}

type SquareImage struct {
	X150x150   string   `json:"150x150"`
	X480x480   string   `json:"480x480"`
	X1000x1000 string   `json:"1000x1000"`
	Mirrors    []string `json:"mirrors"`
}

type PlaylistContents struct {
	TrackIDs []struct {
		Time         int64 `json:"time"`
		Track        int64 `json:"track"`
		MetadataTime int64 `json:"metadata_time"`
	} `json:"track_ids"`
}

type FolloweeRepost struct {
	RepostItemId string `json:"repost_item_id"`
	RepostType   string `json:"repost_type"`
	UserId       string `json:"user_id"`
	CreatedAt    string `json:"created_at"`
}

type FolloweeFavorite struct {
	FavoriteItemId string `json:"favorite_item_id"`
	FavoriteType   string `json:"favorite_type"`
	UserId         string `json:"user_id"`
	CreatedAt      string `json:"created_at"`
}
