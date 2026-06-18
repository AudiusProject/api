package api

import (
	"strings"
	"unicode"
)

// canonicalGenres maps a collapsed lookup key (see genreKey) to the canonical
// display form for genres that must not be plain title-cased — either because
// they merge separator variants ("hip-hop"/"hiphop" -> "Hip-Hop/Rap") or
// because their conventional casing is not title case (R&B/Soul, EDM, ...).
//
// The canonical output forms mirror go-openaudio's GenreAllowlist
// (pkg/etl/processors/entity_manager/genre_allowlist.go), which is the
// protocol's source of truth for canonical genre spelling. Keeping these in
// sync means the API's normalized output agrees with the form the upstream ETL
// indexer (the genre write path) treats as canonical.
//
// The key is the lowercased, alphanumeric-only form of the input, so every
// punctuation/spacing variant of a genre maps through the same entry:
// "R&B", "r & b", "rnb", "R&B/Soul" all collapse to "R&B/Soul".
var canonicalGenres = map[string]string{
	// Allowlist genres whose canonical spelling differs from naive title case.
	"hiphop":      "Hip-Hop/Rap", // "Hip Hop", "hip-hop", "hiphop"
	"hiphoprap":   "Hip-Hop/Rap", // "Hip-Hop/Rap", "hip hop rap"
	"rb":          "R&B/Soul",    // "r&b", "r & b"
	"rnb":         "R&B/Soul",    // "rnb"
	"randb":       "R&B/Soul",    // "r and b"
	"rbsoul":      "R&B/Soul",    // "R&B/Soul"
	"rnbsoul":     "R&B/Soul",    // "rnb/soul"
	"dnb":         "Drum & Bass",
	"drumandbass": "Drum & Bass",
	"drumbass":    "Drum & Bass", // "Drum & Bass" itself
	"lofi":        "Lo-Fi",

	// Acronyms not in the allowlist, kept only to preserve casing (so they are
	// not title-cased to "Edm"/"Dj").
	"edm": "EDM",
	"dj":  "DJ",
}

// genreKey reduces a genre string to a comparison key: lowercased and stripped
// of every non-alphanumeric rune. This is what makes "Hip Hop", "hip-hop", and
// "hiphop" indistinguishable, so variants collapse to a single canonical form.
func genreKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeGenre collapses genre variants to a single canonical form:
//   - trims surrounding whitespace and collapses internal whitespace runs
//   - maps known special cases via canonicalGenres (R&B/Soul, EDM,
//     Hip-Hop/Rap, ...)
//   - otherwise title-cases the value, preserving internal separators
//     ("deep house" -> "Deep House")
//
// Already-canonical allowlist values pass through unchanged (e.g.
// "Electronic", "R&B/Soul", "Hip-Hop/Rap"). An empty/whitespace-only input
// returns "".
func NormalizeGenre(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	if canon, ok := canonicalGenres[genreKey(trimmed)]; ok {
		return canon
	}
	return titleCaseGenre(trimmed)
}

// titleCaseGenre upper-cases the first letter of each alphabetic run and
// lower-cases the rest, leaving non-letter separators in place. Internal
// whitespace runs are collapsed to a single space. So "ELECTRONIC" ->
// "Electronic", "deep house" -> "Deep House", "hip-hop/rap" -> "Hip-Hop/Rap".
func titleCaseGenre(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	var b strings.Builder
	prevLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			if prevLetter {
				b.WriteRune(unicode.ToLower(r))
			} else {
				b.WriteRune(unicode.ToUpper(r))
			}
			prevLetter = true
		} else {
			b.WriteRune(r)
			prevLetter = false
		}
	}
	return b.String()
}
