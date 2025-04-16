package dbv1

type UsageConditions struct {
	UsdcPurchase *struct {
		Price  float64 `json:"price"`
		Splits []struct {
			UserID     int32   `json:"user_id"`
			Percentage float64 `json:"percentage"`
		} `json:"splits"`
	} `json:"usdc_purchase,omitempty"`

	FollowUserID *int64 `json:"follow_user_id,omitempty"`

	TipUserID *int64 `json:"tip_user_id,omitempty"`

	NftCollection *map[string]any `json:"nft_collection,omitempty"`
}

type FullUsageConditions struct {
	UsageConditions
	UsdcPurchase *FullUsdcPurchase `json:"usdc_purchase,omitempty"`
}

type FullUsdcPurchase struct {
	Price  float64            `json:"price"`
	Splits map[string]float64 `json:"splits"`
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

type RemixOf struct {
	Tracks []struct {
		HasRemixAuthorReposted bool  `json:"has_remix_author_reposted"`
		HasRemixAuthorSaved    bool  `json:"has_remix_author_saved"`
		ParentTrackId          int32 `json:"parent_track_id"`
		ParentUserId           int32 `json:"parent_user_id"`
	}
}

type FullRemixOfTrack struct {
	HasRemixAuthorReposted bool     `json:"has_remix_author_reposted"`
	HasRemixAuthorSaved    bool     `json:"has_remix_author_saved"`
	ParentTrackId          string   `json:"parent_track_id"`
	User                   FullUser `json:"user"`
}

type FullRemixOf struct {
	Tracks []FullRemixOfTrack `json:"tracks"`
}
