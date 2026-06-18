package api

import (
	"strings"
	"unicode"
)

// canonicalGenres maps a collapsed lookup key (see genreKey) to the canonical
// display form for genres that must not be plain title-cased — either because
// they merge separator variants ("hip-hop"/"hiphop" -> "Hip Hop") or because
// their conventional casing is not title case (R&B, EDM, DJ, ...).
//
// The key is the lowercased, alphanumeric-only form of the input, so every
// punctuation/spacing variant of a genre maps through the same entry:
// "R&B", "r & b", "rnb" all collapse to key "rb"/"rnb" -> "R&B".
var canonicalGenres = map[string]string{
	"hiphop":      "Hip Hop",
	"rb":          "R&B", // "r&b", "r & b"
	"rnb":         "R&B", // "rnb"
	"randb":       "R&B", // "r and b"
	"edm":         "EDM",
	"dj":          "DJ",
	"dnb":         "Drum & Bass",
	"drumandbass": "Drum & Bass",
	"drumbass":    "Drum & Bass",
	"lofi":        "Lo-Fi",
	"kpop":        "K-Pop",
	"jpop":        "J-Pop",
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
//   - maps known special cases via canonicalGenres (R&B, EDM, Hip Hop, ...)
//   - otherwise title-cases the value, preserving internal separators
//     ("hip-hop/rap" -> "Hip-Hop/Rap")
//
// Already-canonical values pass through unchanged (e.g. "Electronic",
// "R&B", "Hip Hop"). An empty/whitespace-only input returns "".
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
