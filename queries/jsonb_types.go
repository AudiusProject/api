package queries

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
