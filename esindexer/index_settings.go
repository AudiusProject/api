package esindexer

import "github.com/qjebbs/go-jsons"

func commonIndexSettings(mapping string) string {
	if mapping == "" {
		mapping = "{}"
	}

	baseSettings := `
	{
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 0,
			"analysis": {
				"tokenizer": {
					"ngram_tokenizer": {
						"type": "ngram",
						"min_gram": 2,
						"max_gram": 3,
						"token_chars": ["letter", "digit"]
					}
				},
				"analyzer": {
					"infix_analyzer": {
						"tokenizer": "ngram_tokenizer",
						"filter": ["lowercase"]
					}
				}
			}
		},
		"mappings": {
			"properties": {
				"suggest": {
					        "type": "text",
						"analyzer": "infix_analyzer",
						"search_analyzer": "infix_analyzer"
				}
			}
		}
	}
	`

	final, err := jsons.Merge([]byte(baseSettings), []byte(mapping))
	if err != nil {
		panic(err)
	}

	return string(final)
}
