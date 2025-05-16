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
				'saveCount', aggregate_track.save_count,
				'repostCount', aggregate_track.repost_count,
				'commentCount', aggregate_track.comment_count,
				'releaseDate', coalesce(release_date, tracks.created_at),
				'musicalKey', musical_key,
				'bpm', bpm,
				'user', json_build_object(
					'handle', users.handle,
					'name', users.name,
					'location', users.location,
					'followerCount', aggregate_user.follower_count
				)
			)
		FROM tracks
		JOIN aggregate_track USING (track_id)
		JOIN users ON owner_id = user_id
		JOIN aggregate_user USING (user_id)
		LIMIT 10000`

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
