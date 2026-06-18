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
		{"collapses internal whitespace", "Hip   Hop", "Hip Hop"},
		{"empty stays empty", "", ""},
		{"whitespace-only stays empty", "   ", ""},

		// casing
		{"lowercases to title case", "electronic", "Electronic"},
		{"uppercases to title case", "ELECTRONIC", "Electronic"},
		{"mixed case to title case", "eLeCtRoNiC", "Electronic"},
		{"multi-word title case", "deep house", "Deep House"},

		// hip-hop / hiphop variants collapse to "Hip Hop"
		{"hyphenated hip-hop", "hip-hop", "Hip Hop"},
		{"squashed hiphop", "hiphop", "Hip Hop"},
		{"spaced hip hop lowercase", "hip hop", "Hip Hop"},
		{"uppercase HIP-HOP", "HIP-HOP", "Hip Hop"},

		// r&b variants collapse to "R&B"
		{"r&b lowercase", "r&b", "R&B"},
		{"rnb", "rnb", "R&B"},
		{"r & b spaced", "R & B", "R&B"},

		// other special cases keep conventional casing
		{"edm", "edm", "EDM"},
		{"dj", "DJ", "DJ"},
		{"drum and bass", "drum and bass", "Drum & Bass"},
		{"dnb", "dnb", "Drum & Bass"},

		// already-correct values pass through unchanged
		{"Electronic unchanged", "Electronic", "Electronic"},
		{"R&B unchanged", "R&B", "R&B"},
		{"Hip Hop unchanged", "Hip Hop", "Hip Hop"},
		{"EDM unchanged", "EDM", "EDM"},
		{"hyphen/slash genre preserved", "Hip-Hop/Rap", "Hip-Hop/Rap"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeGenre(tc.in), "NormalizeGenre(%q)", tc.in)
		})
	}
}
