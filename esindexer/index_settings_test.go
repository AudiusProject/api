package esindexer

import (
	"testing"

	"bridgerton.audius.co/utils"
)

func TestCommonIndexSettings(t *testing.T) {
	{
		mapping := `{
			"mappings": {
				"properties": {
					"name": {
						"type": "text"
					},
					"handle": {
						"type": "text"
					},
					"location": {
						"type": "text"
					},
					"follower_count": {
						"type": "integer"
					},
					"track_count": {
						"type": "integer"
					}
				}
			}
		}`

		m2 := commonIndexSettings(mapping)

		utils.JsonAssert(t, []byte(m2), map[string]any{
			"mappings.properties.name.type":    "text",
			"mappings.properties.suggest.type": "search_as_you_type",
			"settings.number_of_shards":        1,
			"settings.number_of_replicas":      0,
		})
	}

	// works with empty mapping
	{
		m2 := commonIndexSettings(``)
		utils.JsonAssert(t, []byte(m2), map[string]any{
			"settings.number_of_shards":        1,
			"settings.number_of_replicas":      0,
			"mappings.properties.suggest.type": "search_as_you_type",
		})
	}
}
