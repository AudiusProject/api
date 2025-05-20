package dbv1

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
		Time         float64 `json:"time"`
		Track        int64   `json:"track"`
		MetadataTime float64 `json:"metadata_time"`
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

type EventData struct {
	PrizeInfo   string `json:"prize_info"`
	Description string `json:"description"`
}
