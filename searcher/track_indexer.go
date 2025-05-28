package searcher

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TrackIndexer struct {
	*BaseIndexer
}

func (ti *TrackIndexer) createIndex(drop bool) error {
	// we rely on elasticsearch dynamic mapping
	// so we only need to specify properties that might be ambigious (int vs float)
	mapping := `{
		"mappings": {
			"properties": {
				"bpm":      { "type": "float" }
			}
		}
	}`
	return ti.BaseIndexer.createIndex("tracks", mapping, drop)
}

func (ti *TrackIndexer) indexAll() error {
	sql := `
		SELECT
			track_id,
			json_build_object(
				'title', title,
				'genre', genre,
				'mood', mood,
				'duration', duration,
				'save_count', aggregate_track.save_count,
				'repost_count', aggregate_track.repost_count,
				'comment_count', aggregate_track.comment_count,
				'release_date', coalesce(release_date, tracks.created_at),
				'musical_key', musical_key,
				'bpm', bpm,
				'user', json_build_object(
					'handle', users.handle,
					'name', users.name,
					'location', users.location,
					'follower_count', aggregate_user.follower_count
				)
			)
		FROM tracks
		JOIN aggregate_track USING (track_id)
		JOIN users ON owner_id = user_id
		JOIN aggregate_user USING (user_id)
		WHERE track_id < 100000
		LIMIT 100000`

	return ti.bulkIndexQuery("tracks", sql)
}

func (ti *TrackIndexer) search(q string) {
	query := fmt.Sprintf(`{
		"query": {
			"multi_match": {
				"query": %q,
				"fields": ["title", "genre", "mood"]
			}
		}
	}`, q)

	fmt.Println(query)

	res, err := ti.esc.Search(
		ti.esc.Search.WithIndex("tracks"),
		ti.esc.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		fmt.Println("es search error:", err)
		return
	}
	defer res.Body.Close()

	type hit struct {
		ID string `json:"_id"`
	}
	type hits struct {
		Hits struct {
			Hits []hit `json:"hits"`
		} `json:"hits"`
	}

	var result hits
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		fmt.Println("decode error:", err)
		return
	}

	var ids []string
	for _, h := range result.Hits.Hits {
		ids = append(ids, h.ID)
	}

	fmt.Println("found ids:", ids)

}
