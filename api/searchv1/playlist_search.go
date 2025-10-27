package searchv1

import (
	"fmt"
	"strings"

	"github.com/aquasecurity/esquery"
)

type PlaylistSearchQuery struct {
	Query        string
	Genres       []string
	Moods        []string
	IsTagSearch  bool
	IsAlbum      bool
	OnlyVerified bool
	MyID         int32
	SortMethod   string
}

func (q *PlaylistSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if q.IsTagSearch {
		builder.Must(esquery.MultiMatch().Query(q.Query).Fields("tracks.tags").Type(esquery.MatchTypeBoolPrefix))
	} else if q.Query != "" {
		builder.Must(esquery.MultiMatch().
			Query(q.Query).
			Fields("title^10", "suggest", "tracks.tags").
			MinimumShouldMatch("100%").
			Fuzziness("AUTO").
			Type(esquery.MatchTypeBoolPrefix))

		// for exact title / handle / artist name match
		builder.Should(
			esquery.MultiMatch().Query(q.Query).
				Fields("title^10", "user.name", "user.handle").
				Boost(10).
				Operator(esquery.OperatorAnd),
		)

		// exact match, but remove spaces from query
		builder.Should(
			esquery.MultiMatch().Query(strings.ReplaceAll(q.Query, " ", "")).
				Fields("title^10", "user.name", "user.handle").
				Boost(10).
				Operator(esquery.OperatorAnd),
		)
	} else {
		builder.Must(esquery.MatchAll())
	}

	builder.Filter(esquery.Term("is_album", q.IsAlbum))

	if len(q.Genres) > 0 {
		// Normalize to capitalized names since keyword filtering must be exact
		capitalizedGenres := make([]string, len(q.Genres))
		for i, value := range q.Genres {
			capitalizedGenres[i] = strings.ToUpper(string(value[0])) + strings.ToLower(value[1:])
		}
		builder.Filter(esquery.Terms("tracks.genre.keyword", toAnySlice(capitalizedGenres)...))
		// by using a match query... the TF/IDF will apply to tracks.  Which will rank playlists higher if they have a larger proportion of genre
		for _, value := range capitalizedGenres {
			builder.Should(esquery.Match("tracks.genre", value)).Boost(10)
		}
	}

	if len(q.Moods) > 0 {
		// Normalize to capitalized names since keyword filtering must be exact
		capitalizedMoods := make([]string, len(q.Moods))
		for i, value := range q.Moods {
			capitalizedMoods[i] = strings.ToUpper(string(value[0])) + strings.ToLower(value[1:])
		}
		builder.Filter(esquery.Terms("tracks.mood.keyword", toAnySlice(capitalizedMoods)...))
		// by using a match query... the TF/IDF will apply to tracks.  Which will rank playlists higher if they have a larger proportion of mood
		for _, value := range capitalizedMoods {
			builder.Should(esquery.Match("tracks.mood", value)).Boost(10)
		}
	}

	if q.OnlyVerified {
		builder.Must(esquery.Term("tracks.user.is_verified", true))
	} else {
		builder.Should(esquery.Term("user.is_verified", true))
	}

	// personalize results
	if q.MyID > 0 {
		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "reposted_playlist_ids",
				},
				"boost": 1000,
			},
		}))

		builder.Should(esquery.CustomQuery(map[string]any{
			"terms": map[string]any{
				"_id": map[string]any{
					"index": "socials",
					"id":    fmt.Sprintf("%d", q.MyID),
					"path":  "saved_playlist_ids",
				},
				"boost": 1000,
			},
		}))
	}

	return builder.Map()
}

func (q *PlaylistSearchQuery) DSL() string {
	switch q.SortMethod {
	case "recent":
		return sortWithField(q.Map(), "created_at", "desc")
	case "popular":
		return BuildFunctionScoreDSL("repost_count", 200, q.Map())
	default:
		return BuildFunctionScoreDSL("repost_count", 20, q.Map())
	}
}
