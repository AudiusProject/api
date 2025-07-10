package searchv1

import (
	"fmt"
	"strings"

	"github.com/aquasecurity/esquery"
)

type TrackSearchQuery struct {
	Query          string
	MinBPM         int
	MaxBPM         int
	IsDownloadable bool
	IsPurchaseable bool
	IsTagSearch    bool
	HasDownloads   bool
	OnlyVerified   bool
	Genres         []string
	Moods          []string
	MusicalKeys    []string
	MyID           int32
	SortMethod     string
}

func (q *TrackSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.IsTagSearch {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(
			esquery.MultiMatch().
				Query(q.Query).
				Fields("title^10", "suggest", "tags").
				MinimumShouldMatch("100%").
				Fuzziness("AUTO").
				Type(esquery.MatchTypeBoolPrefix),
		)

		// for exact title / handle / artist name match
		builder.Should(
			esquery.MultiMatch().Query(q.Query).
				Fields("title^10", "user.name", "user.handle").
				Boost(10).
				Operator(esquery.OperatorAnd),
		)

		// exact match, but remove spaces from query
		// so 'Pure Component' ranks 'PureComponent' higher
		builder.Should(
			esquery.MultiMatch().Query(strings.ReplaceAll(q.Query, " ", "")).
				Fields("title^10", "user.name", "user.handle").
				Boost(10).
				Operator(esquery.OperatorAnd),
		)
	} else {
		builder.Must(esquery.MatchAll())
	}

	if q.MinBPM > 0 || q.MaxBPM > 0 {
		bpmRange := esquery.Range("bpm")
		if q.MinBPM > 0 {
			bpmRange.Gte(q.MinBPM)
		}
		if q.MaxBPM > 0 {
			bpmRange.Lte(q.MaxBPM)
		}
		builder.Filter(bpmRange)
	}

	if len(q.Genres) > 0 {
		builder.Filter(esquery.Terms("genre.keyword", toAnySlice(q.Genres)...))
	}

	if len(q.Moods) > 0 {
		builder.Filter(esquery.Terms("mood.keyword", toAnySlice(q.Moods)...))
	}

	if len(q.MusicalKeys) > 0 {
		builder.Filter(esquery.Terms("musical_key.keyword", toAnySlice(q.MusicalKeys)...))
	}

	if q.IsDownloadable {
		builder.Filter(esquery.Term("is_downloadable", true))
	}

	if q.HasDownloads {
		builder.Filter(esquery.Bool().Should(
			esquery.Term("is_downloadable", true),
			esquery.Term("has_stems", true),
		))
	}

	if q.IsPurchaseable {
		// stream or download
		builder.Filter(
			esquery.Bool().
				Should(esquery.Exists("stream_conditions.usdc_purchase")).
				Should(esquery.Exists("download_conditions.usdc_purchase")),
		)
	}

	if q.OnlyVerified {
		builder.Must(esquery.Term("user.is_verified", true))
	} else {
		builder.Should(esquery.Term("user.is_verified", true))
	}

	// boost tracks that are saved / reposted
	if q.MyID > 0 {
		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "reposted_track_ids",
				},
				"boost": 1000,
			},
		}))

		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "saved_track_ids",
				},
				"boost": 1000,
			},
		}))
	}

	return builder.Map()
}

func (q *TrackSearchQuery) DSL() string {
	switch q.SortMethod {
	case "recent":
		return sortWithField(q.Map(), "created_at", "desc")
	case "popular":
		return BuildFunctionScoreDSL("play_count", 200, q.Map())
	default:
		return BuildFunctionScoreDSL("repost_count", 20, q.Map())
	}
}
