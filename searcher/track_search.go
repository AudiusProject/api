package searcher

import (
	"fmt"

	"github.com/aquasecurity/esquery"
)

type TrackSearchQuery struct {
	Query          string
	MinBPM         int
	MaxBPM         int
	IsDownloadable bool
	IsPurchaseable bool
	Genres         []string
	Moods          []string
	MusicalKeys    []string
	MyID           int32
}

func (t *TrackSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if t.Query != "" {
		builder.Must(esquery.Match("title", t.Query))
	} else {
		builder.Must(esquery.MatchAll())
	}

	if t.MinBPM > 0 || t.MaxBPM > 0 {
		bpmRange := esquery.Range("bpm")
		if t.MinBPM > 0 {
			bpmRange.Gte(t.MinBPM)
		}
		if t.MaxBPM > 0 {
			bpmRange.Lte(t.MaxBPM)
		}
		builder.Filter(bpmRange)
	}

	if len(t.Genres) > 0 {
		builder.Filter(esquery.Terms("genre", toAnySlice(t.Genres)...))
	}

	if len(t.Moods) > 0 {
		builder.Filter(esquery.Terms("mood", toAnySlice(t.Moods)...))
	}

	if len(t.MusicalKeys) > 0 {
		builder.Filter(esquery.Terms("musical_key.keyword", toAnySlice(t.MusicalKeys)...))
	}

	if t.IsDownloadable {
		builder.Filter(esquery.Term("is_downloadable", true))
	}

	if t.IsPurchaseable {
		// stream or download
		builder.Filter(
			esquery.Bool().
				Should(esquery.Exists("stream_conditions.usdc_purchase")).
				Should(esquery.Exists("download_conditions.usdc_purchase")),
		)
	}

	// boost tracks that are saved / reposted
	if t.MyID > 0 {
		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", t.MyID),
					"path":  "reposted_track_ids",
				},
				"boost": 10,
			},
		}))
	}

	return builder.Map()
}
