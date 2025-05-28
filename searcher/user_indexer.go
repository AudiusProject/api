package searcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

/*

stage:
http://35.239.100.60:21302

prod:
http://35.238.44.255:21302

*/

type UserIndexer struct {
	*BaseIndexer
}

func (ui *UserIndexer) createIndex(drop bool) error {
	mapping := ``
	return ui.BaseIndexer.createIndex("users", mapping, drop)
}

func (ui *UserIndexer) indexAll() error {
	sql := `
		SELECT
			user_id,
			json_build_object(
				'name', name,
				'handle', handle,
				'bio', bio,
				'location', location,
				'created_at', created_at,

				'track_count', track_count,
				'playlist_count', playlist_count,
				'follower_count', follower_count,
				'repost_count', repost_count,
				'track_save_count', track_save_count,
				'supporter_count', supporter_count,
				'supporting_count', supporting_count,
				'dominant_genre', dominant_genre,
				'dominant_genre_count', dominant_genre_count
			)
		FROM users
		JOIN aggregate_user USING (user_id)
		LIMIT 10000`

	return ui.bulkIndexQuery("users", sql)
}

func (ui *UserIndexer) search(q string) {
	query := fmt.Sprintf(`{
		"query": {
			"simple_query_string": {
				"query": %q,
				"default_operator": "AND"
			}
		}
	}`, q+"*")

	req := esapi.SearchRequest{
		Index: []string{"users"},
		Body:  strings.NewReader(query),
	}

	res, err := req.Do(context.Background(), ui.esc)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Error: %s", res.String())
	}

	// Print the response body
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}

	// Print the search results
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		fmt.Printf("Document: %v\n", hit)
	}
}
