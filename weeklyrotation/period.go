// Package weeklyrotation holds the calendar arithmetic behind the Weekly
// Rotation mix, shared by the endpoint that computes the mix
// (api/v1_users_weekly_rotation.go) and the job that announces it
// (jobs/create_weekly_rotation_notifications.go). Both need to agree on
// exactly when a period begins, so the definition lives in one place.
//
// The web client has the same logic in
// packages/web/src/utils/weeklyRotationPeriod.ts with the same test cases;
// change both together.
package weeklyrotation

import (
	"fmt"
	"time"
)

// RolloverOffsetDays is how far the period boundary sits after the ISO
// week's Monday. The mix rolls over on Wednesday 00:00 UTC, not at the ISO
// week boundary: Monday already belongs to the other weekly surfaces and
// Friday is Spotify's day. Expressed as an offset so the period is still
// identified by an (iso_year, iso_week) pair everywhere -- cache keys, the
// deterministic seed, share links, notification group ids.
const RolloverOffsetDays = 2

// Period returns the (ISO year, ISO week) pair that identifies the period
// containing t. Shifting t back by the rollover offset before taking the
// ISO week means Wednesday..Tuesday map to one ISO week.
func Period(t time.Time) (year, week int) {
	return t.UTC().AddDate(0, 0, -RolloverOffsetDays).ISOWeek()
}

// PeriodStart is the inverse of Period: the instant the (year, week) period
// began, i.e. Wednesday 00:00 UTC of that ISO week.
func PeriodStart(year, week int) time.Time {
	// ISO week 1 is the week containing January 4th.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday: Go says 0, ISO says 7
	}
	mondayOfWeek1 := jan4.AddDate(0, 0, -(weekday - 1))
	monday := mondayOfWeek1.AddDate(0, 0, (week-1)*7)
	return monday.AddDate(0, 0, RolloverOffsetDays)
}

// PeriodKey renders a period as "YYYY-WW", the same form the og card's
// `week` query param and the notification group ids use.
func PeriodKey(year, week int) string {
	return fmt.Sprintf("%d-%02d", year, week)
}
