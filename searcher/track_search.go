package searcher

import (
	"github.com/aquasecurity/esquery"
)

type TrackSearchQuery struct {
	Query       string
	MinBPM      int
	MaxBPM      int
	Genres      []string
	Moods       []string
	MusicalKeys []string
}

func (t *TrackSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if t.Query != "" {
		builder.Must(esquery.Match("title", t.Query))
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
		builder.Filter(esquery.Match("genre", toAnySlice(t.Genres)...))
	}

	if len(t.Moods) > 0 {
		builder.Filter(esquery.Match("mood", toAnySlice(t.Moods)...))
	}

	if len(t.MusicalKeys) > 0 {
		builder.Filter(esquery.Terms("musical_key.keyword", toAnySlice(t.MusicalKeys)...))
	}

	return builder.Map()
}
