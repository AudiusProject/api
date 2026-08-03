package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenreNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// trimming
		{"trims surrounding whitespace", "  Electronic  ", "Electronic"},
		{"collapses internal whitespace", "Deep   House", "Deep House"},
		{"empty stays empty", "", ""},
		{"whitespace-only stays empty", "   ", ""},

		// casing
		{"lowercases to title case", "electronic", "Electronic"},
		{"uppercases to title case", "ELECTRONIC", "Electronic"},
		{"mixed case to title case", "eLeCtRoNiC", "Electronic"},
		{"multi-word title case", "deep house", "Deep House"},

		// hip-hop / hiphop variants collapse to "Hip Hop"
		// hip-hop / hiphop variants collapse to the allowlist form "Hip-Hop/Rap"
		{"hyphenated hip-hop", "hip-hop", "Hip-Hop/Rap"},
		{"squashed hiphop", "hiphop", "Hip-Hop/Rap"},
		{"spaced hip hop lowercase", "hip hop", "Hip-Hop/Rap"},
		{"uppercase HIP-HOP", "HIP-HOP", "Hip-Hop/Rap"},

		// r&b variants collapse to the allowlist form "R&B/Soul"
		{"r&b lowercase", "r&b", "R&B/Soul"},
		{"rnb", "rnb", "R&B/Soul"},
		{"r & b spaced", "R & B", "R&B/Soul"},

		// other special cases keep conventional casing
		{"edm", "edm", "EDM"},
		{"dj", "DJ", "DJ"},
		{"drum and bass", "drum and bass", "Drum & Bass"},
		{"dnb", "dnb", "Drum & Bass"},

		// already-canonical allowlist values pass through unchanged
		{"Electronic unchanged", "Electronic", "Electronic"},
		{"Hip-Hop/Rap unchanged", "Hip-Hop/Rap", "Hip-Hop/Rap"},
		{"R&B/Soul unchanged", "R&B/Soul", "R&B/Soul"},
		{"Drum & Bass unchanged", "Drum & Bass", "Drum & Bass"},
		{"Lo-Fi unchanged", "Lo-Fi", "Lo-Fi"},
		{"Deep House unchanged", "Deep House", "Deep House"},
		{"EDM unchanged", "EDM", "EDM"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeGenre(tc.in), "NormalizeGenre(%q)", tc.in)
		})
	}
}
