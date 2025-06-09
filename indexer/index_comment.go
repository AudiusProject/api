package indexer

import (
	"encoding/json"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/jackc/pgx/v5"
)

func (ci *CoreIndexer) createComment(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var metadata GenericMetadata
	if err := json.Unmarshal([]byte(em.Metadata), &metadata); err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"comment_id":  metadata.Data["comment_id"],
		"user_id":     em.UserId,
		"text":        metadata.Data["body"],
		"entity_type": "track",
		"entity_id":   metadata.Data["track_id"],

		"txhash":      txInfo.txhash,
		"blockhash":   txInfo.blockhash,
		"blocknumber": txInfo.blocknumber,
	}

	return ci.doInsert("comments", args)
}
