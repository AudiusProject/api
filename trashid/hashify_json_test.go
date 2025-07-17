package trashid

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestHashifyJson(t *testing.T) {
	j1 := []byte(`
	{
		"data": {
			"id": 1,
			"user_id": 2,
			"special_id": 999,
			"tracks": [
				{
					"id": 3,
					"title": "fun",
					"value": "id",
					"other": "user_id",
					"good_idea": 333,
					"string_id": "333",
					"fake_string_id": "333_444",
					"nested": {
						"string_id": "333",
						"fake_string_id": "333_444"
					},
					"ida": 111
				},
				{
					"id": 4,
					"user_id": 1,
					"title": "fun",
					"value": "id",
					"other": "user_id",
					"good_idea": 333,
					"ida": 111
				}
			]
		}
	}
	`)

	var m map[string]any
	err := json.Unmarshal(j1, &m)
	assert.NoError(t, err)

	j2 := HashifyJson(j1)

	expectations := map[string]string{
		"data.id":                             "7eP5n",
		"data.user_id":                        "ML51L",
		"data.special_id":                     "999",
		"data.tracks.0.id":                    "lebQD",
		"data.tracks.0.value":                 "id",
		"data.tracks.0.other":                 "user_id",
		"data.tracks.0.good_idea":             "333",
		"data.tracks.0.string_id":             "LjzGL",
		"data.tracks.0.fake_string_id":        "333_444",
		"data.tracks.0.nested.string_id":      "LjzGL",
		"data.tracks.0.nested.fake_string_id": "333_444",
		"data.tracks.1.id":                    "ELKzn",
		"data.tracks.1.user_id":               "7eP5n",
	}
	for path, exp := range expectations {
		assert.Equal(t, exp, gjson.GetBytes(j2, path).String(), "for path "+path)
	}

	err = json.Unmarshal(j2, &m)
	assert.NoError(t, err)

}
