package api

import (
	"testing"
	"time"
)

// parseTime parses a time string using RFC3339 format and fails the test if parsing fails
func parseTime(t *testing.T, timeStr string) time.Time {
	return parseTimeWithLayout(t, timeStr, time.DateOnly)
}

// parseTimeWithLayout parses a time string with the given layout and fails the test if parsing fails
func parseTimeWithLayout(t *testing.T, timeStr string, layout string) time.Time {
	t.Helper()
	parsed, err := time.Parse(layout, timeStr)
	if err != nil {
		t.Fatalf("Failed to parse time string %q: %v", timeStr, err)
	}
	return parsed
}
