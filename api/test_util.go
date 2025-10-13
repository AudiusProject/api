package api

import (
	"encoding/csv"
	"strings"
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

// parseCSVLines parses CSV content and returns the header row and data rows
func parseCSVLines(csvContent string) ([]string, [][]string, error) {
	reader := csv.NewReader(strings.NewReader(csvContent))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}

	if len(records) == 0 {
		return nil, nil, nil
	}

	header := records[0]
	data := records[1:]
	return header, data, nil
}
