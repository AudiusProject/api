package api

import "time"

type CommsSummary struct {
	TotalCount  int64     `db:"total_count" json:"total_count"`
	BeforeCount int64     `db:"before_count" json:"prev_count"`
	AfterCount  int64     `db:"after_count" json:"next_count"`
	Prev        time.Time `db:"prev" json:"prev_cursor"`
	Next        time.Time `db:"next" json:"next_cursor"`
}

type CommsHealth struct {
	IsHealthy bool `json:"is_healthy"`
}

type CommsResponse struct {
	Data    any           `json:"data"`
	Summary *CommsSummary `json:"summary,omitempty"`
	Health  CommsHealth   `json:"health"`
}
