package searcher

import (
	"encoding/json"
	"fmt"
	"strings"
)

type PlaylistIndexer struct {
	*BaseIndexer
}

func (pi *PlaylistIndexer) createIndex(drop bool) error {
	// Specify mappings for playlist index
	// Only specifying properties that might be ambiguous
	mapping := `{
		"mappings": {
			"properties": {
			}
		}
	}`
	return pi.BaseIndexer.createIndex("playlists", mapping, drop)
}

func (pi *PlaylistIndexer) indexAll() error {
	sql := `
		SELECT
			playlist_id,
			json_build_object(
				'title', playlist_name,
				'description', description,
				'track_count', track_count,
				'save_count', aggregate_playlist.save_count,
				'repost_count', aggregate_playlist.repost_count,
				'last_updated', playlists.updated_at,
				'created_at', playlists.created_at,
				'is_private', is_private,
				'is_album', playlists.is_album,
				'user', json_build_object(
					'handle', users.handle,
					'name', users.name,
					'location', users.location,
					'follower_count', aggregate_user.follower_count
				)
			)
		FROM playlists
		JOIN aggregate_playlist USING (playlist_id)
		JOIN users ON playlist_owner_id = user_id
		JOIN aggregate_user USING (user_id)
		LIMIT 10000`

	return pi.bulkIndexQuery("playlists", sql)
}

func (pi *PlaylistIndexer) search(q string) {
	query := fmt.Sprintf(`{
		"query": {
			"multi_match": {
				"query": %q,
				"fields": ["title", "description"]
			}
		}
	}`, q)

	fmt.Println(query)

	res, err := pi.esc.Search(
		pi.esc.Search.WithIndex("playlists"),
		pi.esc.Search.WithBody(strings.NewReader(query)),
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
