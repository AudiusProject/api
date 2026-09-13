package weeklyrotation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriod(t *testing.T) {
	utc := func(y int, m time.Month, d, h int) time.Time {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
	}

	// 2026-09-09 is a Wednesday.
	rollover := utc(2026, time.September, 9, 0)

	y, w := Period(rollover)
	assert.Equal(t, [2]int{2026, 37}, [2]int{y, w}, "the rollover instant opens ISO week 37's period")
	assert.Equal(t, rollover, PeriodStart(y, w))

	y, w = Period(rollover.Add(-time.Second))
	assert.Equal(t, [2]int{2026, 36}, [2]int{y, w}, "one second earlier is still the previous period")
	assert.Equal(t, utc(2026, time.September, 2, 0), PeriodStart(y, w))

	y, w = Period(utc(2026, time.September, 7, 12)) // the Monday
	assert.Equal(t, [2]int{2026, 36}, [2]int{y, w}, "Monday and Tuesday belong to the period that started the previous Wednesday")

	// Non-UTC input is normalised: 2026-09-08 20:00 PDT is 2026-09-09 03:00 UTC.
	pdt := time.FixedZone("PDT", -7*3600)
	y, w = Period(time.Date(2026, time.September, 8, 20, 0, 0, 0, pdt))
	assert.Equal(t, [2]int{2026, 37}, [2]int{y, w})

	// Year boundary: ISO week 1 of 2027 starts Monday 2027-01-04, so its
	// period starts Wednesday 2027-01-06, and the days before that belong
	// to 2026's last ISO week (53).
	y, w = Period(utc(2027, time.January, 6, 0))
	assert.Equal(t, [2]int{2027, 1}, [2]int{y, w})
	assert.Equal(t, utc(2027, time.January, 6, 0), PeriodStart(2027, 1))

	y, w = Period(utc(2027, time.January, 5, 23))
	assert.Equal(t, [2]int{2026, 53}, [2]int{y, w})
	assert.Equal(t, utc(2026, time.December, 30, 0), PeriodStart(2026, 53))

	// Round trip across a whole year of hours.
	for tm := utc(2026, time.January, 1, 0); tm.Before(utc(2027, time.January, 1, 0)); tm = tm.Add(time.Hour) {
		py, pw := Period(tm)
		start := PeriodStart(py, pw)
		require.False(t, tm.Before(start), "%v is before its own period start %v", tm, start)
		require.True(t, tm.Before(start.AddDate(0, 0, 7)), "%v is past the end of its period starting %v", tm, start)
	}
}

func TestPeriodKey(t *testing.T) {
	assert.Equal(t, "2026-37", PeriodKey(2026, 37))
	assert.Equal(t, "2027-01", PeriodKey(2027, 1), "week is zero-padded to match the og card's week param")
}
