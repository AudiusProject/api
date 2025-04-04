package indexer

import (
	"strings"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/jackc/pgx/v5"
)

func (ci *CoreIndexer) followUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	args := pgx.NamedArgs{
		"blockhash":        txInfo.blockhash,
		"blocknumber":      txInfo.blocknumber,
		"follower_user_id": em.UserId,
		"followee_user_id": em.EntityId,
		"is_current":       true,
		"is_delete":        false,
		"created_at":       txInfo.timestamp,
		"txhash":           txInfo.txhash,
		"slot":             500, // TODO
	}
	return ci.doInsert("follows", args)
}

func (ci *CoreIndexer) unfollowUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	return ci.doUpdate("follows",
		pgx.NamedArgs{
			"is_delete": true,
		},
		pgx.NamedArgs{
			"follower_user_id": em.UserId,
			"followee_user_id": em.EntityId,
		})
}

func (ci *CoreIndexer) repost(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	args := pgx.NamedArgs{
		"blockhash":           txInfo.blockhash,
		"blocknumber":         txInfo.blocknumber,
		"user_id":             em.UserId,
		"repost_item_id":      em.EntityId,
		"repost_type":         strings.ToLower(em.EntityType),
		"is_current":          true,
		"is_delete":           false,
		"created_at":          txInfo.timestamp,
		"txhash":              txInfo.txhash,
		"slot":                500,
		"is_repost_of_repost": false, // todo
	}
	return ci.doInsert("reposts", args)
}

func (ci *CoreIndexer) favorite(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	args := pgx.NamedArgs{
		"blockhash":    txInfo.blockhash,
		"blocknumber":  txInfo.blocknumber,
		"user_id":      em.UserId,
		"save_item_id": em.EntityId,
		"save_type":    strings.ToLower(em.EntityType),
		"is_current":   true,
		"is_delete":    false,
		"created_at":   txInfo.timestamp,
		"txhash":       txInfo.txhash,
		"slot":         500,
	}
	return ci.doInsert("saves", args)
}

func (ci *CoreIndexer) unfavorite(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	args := pgx.NamedArgs{
		"blockhash":   txInfo.blockhash,
		"blocknumber": txInfo.blocknumber,
		"is_delete":   true,
		"txhash":      txInfo.txhash,
		"slot":        500, // TODO
	}
	return ci.doUpdate("saves", args, pgx.NamedArgs{
		"user_id":      em.UserId,
		"save_item_id": em.EntityId,
		"save_type":    strings.ToLower(em.EntityType),
	})
}

func (ci *CoreIndexer) viewNotification(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	args := pgx.NamedArgs{
		"user_id":     em.UserId,
		"seen_at":     txInfo.timestamp,
		"blocknumber": txInfo.blocknumber,
		"blockhash":   txInfo.blockhash,
		"txhash":      txInfo.txhash,
	}
	return ci.doInsert("notification_seen", args)
}
