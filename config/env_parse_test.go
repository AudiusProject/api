package config

import "testing"

// The cutover bounds all mean "no bound" at 0, so a malformed value must not
// read as 0 -- that would silently disable the very limit it was meant to set,
// and the failure would only surface as duplicated or missing rows much later.
func TestMustParseInt64Env(t *testing.T) {
	t.Run("unset is zero", func(t *testing.T) {
		if got := mustParseInt64Env("definitelyNotSetAnywhere"); got != 0 {
			t.Errorf("expected 0 for unset, got %d", got)
		}
	})

	t.Run("parses a value", func(t *testing.T) {
		t.Setenv("someCutoverBound", "24000000")
		if got := mustParseInt64Env("someCutoverBound"); got != 24000000 {
			t.Errorf("expected 24000000, got %d", got)
		}
	})

	t.Run("panics on garbage rather than reading as zero", func(t *testing.T) {
		t.Setenv("someCutoverBound", "24_000_000")
		defer func() {
			if recover() == nil {
				t.Error("expected a panic; a typo must not silently disable the bound")
			}
		}()
		mustParseInt64Env("someCutoverBound")
	})
}
