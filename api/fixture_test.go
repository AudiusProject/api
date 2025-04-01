package api

import (
	"context"
	"encoding/csv"
	"os"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	userBaseRow = map[string]any{
		"user_id":              nil,
		"handle":               nil,
		"handle_lc":            nil,
		"is_current":           true,
		"is_verified":          false,
		"created_at":           time.Now(),
		"updated_at":           time.Now(),
		"has_collectibles":     false,
		"txhash":               "tx1",
		"is_deactivated":       false,
		"is_available":         true,
		"is_storage_v2":        false,
		"allow_ai_attribution": false,
	}

	trackBaseRow = map[string]any{
		"blockhash":                             "block_abc123",
		"track_id":                              "@track_id",
		"is_current":                            true,
		"is_delete":                             false,
		"owner_id":                              "@owner_id",
		"title":                                 "@title",
		"genre":                                 "Electronic",
		"mood":                                  "Energetic",
		"created_at":                            time.Now(),
		"updated_at":                            time.Now(),
		"txhash":                                "tx_123abc",
		"is_unlisted":                           false,
		"is_available":                          true,
		"track_segments":                        "[]", // JSONB string
		"is_scheduled_release":                  false,
		"is_downloadable":                       false,
		"is_original_available":                 false,
		"playlists_containing_track":            "{}", // JSONB string
		"playlists_previously_containing_track": map[string]any{},
		"audio_analysis_error_count":            0,
		"is_owned_by_user":                      false,
	}

	followBaseRow = map[string]any{
		"blockhash":        "block1",
		"blocknumber":      101,
		"follower_user_id": nil,
		"followee_user_id": nil,
		"is_current":       true,
		"is_delete":        false,
		"created_at":       time.Now(),
		"txhash":           "tx123",
		"slot":             500,
	}

	repostBaseRow = map[string]any{
		"blockhash":           "block_abc123",
		"blocknumber":         101,
		"user_id":             nil,
		"repost_item_id":      nil,
		"repost_type":         nil,
		"is_current":          true,
		"is_delete":           false,
		"created_at":          time.Now(),
		"txhash":              "tx_456def",
		"slot":                500,
		"is_repost_of_repost": false,
	}
)

func insertFixtures(table string, baseRow map[string]any, csvFile string) {
	file, err := os.Open(csvFile)
	checkErr(err)
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	checkErr(err)
	csvHeader := rows[0]

	// union baseRow keys with csv header for field list
	fieldList := []string{}
	for f := range baseRow {
		fieldList = append(fieldList, f)
	}
	for _, f := range csvHeader {
		if !slices.Contains(fieldList, f) {
			fieldList = append(fieldList, f)
		}
	}

	var records [][]any
	for _, row := range rows[1:] {
		for i, field := range csvHeader {
			if row[i] != "" {
				baseRow[field] = row[i]
			}
		}

		vals := []any{}
		for _, field := range fieldList {
			vals = append(vals, baseRow[field])
		}
		records = append(records, vals)
	}

	_, err = app.pool.CopyFrom(
		context.Background(),
		pgx.Identifier{table},
		fieldList,
		pgx.CopyFromRows(records),
	)
	checkErr(err)
}
