package indexer

import (
	"encoding/json"
	"strings"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/jackc/pgx/v5"
)

func (ci *CoreIndexer) createUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var metadata GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &metadata)

	args := pgx.NamedArgs{
		"user_id":              em.EntityId,
		"handle_lc":            strings.ToLower(metadata.Data["handle"].(string)),
		"is_current":           true,
		"is_verified":          false,
		"created_at":           txInfo.timestamp,
		"updated_at":           txInfo.timestamp,
		"has_collectibles":     false,
		"txhash":               txInfo.txhash,
		"is_deactivated":       false,
		"is_available":         true,
		"is_storage_v2":        false,
		"allow_ai_attribution": false,
	}

	for k, v := range metadata.Data {
		if k == "events" {
			continue
		}
		args[k] = v
	}

	return ci.doInsert("users", args)
}

func (ci *CoreIndexer) updateUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var metadata GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &metadata)

	args := pgx.NamedArgs{}

	// todo: verify this user is authorized to edit
	// todo: better validation of fields + values

	for k, v := range metadata.Data {
		if k == "events" || k == "handle" {
			continue
		}
		args[k] = v
	}

	if _, exists := args["isDeactivated"]; exists {
		// sometimes metadata is camelCase
		// which breaks doUpdate
		// so silently skip for now...
		// todo: better validation
		return nil
	}

	return ci.doUpdate("users", args, pgx.NamedArgs{
		"user_id": em.EntityId,
	})
}
