package indexer

import (
	"encoding/json"
	"maps"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/jackc/pgx/v5"
)

func (ci *CoreIndexer) createTrack(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var meta GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &meta)

	args := pgx.NamedArgs{
		"blockhash":                             txInfo.blockhash,
		"track_id":                              em.EntityId,
		"is_current":                            true,
		"is_delete":                             false,
		"owner_id":                              em.UserId,
		"title":                                 "@title",
		"created_at":                            txInfo.timestamp,
		"updated_at":                            txInfo.timestamp,
		"txhash":                                txInfo.txhash,
		"is_unlisted":                           false,
		"is_available":                          true,
		"track_segments":                        "[]", // JSONB string
		"is_scheduled_release":                  false,
		"is_downloadable":                       false,
		"is_original_available":                 false,
		"playlists_containing_track":            "{}", // JSONB string
		"playlists_previously_containing_track": map[string]any{},
		"audio_analysis_error_count":            0,
		"is_owned_by_user":                      false,
	}

	for k, v := range meta.Data {
		args[k] = v
	}

	return ci.doInsert("tracks", args)
}

func (ci *CoreIndexer) updateTrack(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var metadata GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &metadata)

	// todo: verify this user is authorized to edit

	args := pgx.NamedArgs{}
	maps.Copy(args, metadata.Data)

	return ci.doUpdate("tracks", args, pgx.NamedArgs{
		"track_id": em.EntityId,
		"owner_id": em.UserId,
	})
}

func (ci *CoreIndexer) deleteTrack(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	return ci.doUpdate("tracks",
		pgx.NamedArgs{
			"is_delete": true,
		},
		pgx.NamedArgs{
			"track_id": em.EntityId,
			"owner_id": em.UserId,
		})
}

func (ci *CoreIndexer) downloadTrack(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var meta GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &meta)

	args := pgx.NamedArgs{
		"blocknumber":     txInfo.blocknumber,
		"txhash":          txInfo.txhash,
		"created_at":      txInfo.timestamp,
		"user_id":         em.UserId,
		"parent_track_id": em.EntityId,
		"track_id":        em.EntityId,
	}

	maps.Copy(args, meta.Data)
	return ci.doInsert("track_downloads", args)
}
