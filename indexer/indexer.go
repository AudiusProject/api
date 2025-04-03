package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CoreIndexer struct {
	ctx  context.Context
	pool *pgxpool.Pool
}

type CoreIndexerConfig struct {
	DbUrl string
}

func NewIndexer(config CoreIndexerConfig) (*CoreIndexer, error) {
	bg := context.Background()
	pool, err := pgxpool.New(bg, config.DbUrl)
	if err != nil {
		return nil, err
	}

	ci := &CoreIndexer{
		bg,
		pool,
	}

	return ci, nil
}

func (ci *CoreIndexer) handleTx(signedTx *core_proto.SignedTransaction) error {

	// txInfo contains some context around the tx:
	// blockhash + blocknumber
	// also need block timestamp
	// todo: where best place to get?
	txInfo := TxInfo{
		txhash:      signedTx.TxHash(),
		blockhash:   "todo",
		blocknumber: 123,
	}

	switch signedTx.GetTransaction().(type) {
	case *core_proto.SignedTransaction_Plays:
		// play := signedTx.GetPlays()
		// fmt.Println("PLAY", play)

	case *core_proto.SignedTransaction_ManageEntity:
		em := signedTx.GetManageEntity()
		action := em.Action + em.EntityType
		var err error

		switch action {
		case "CreateUser":
			err = ci.createUser(txInfo, em)
		case "CreateTrack":
			err = ci.createTrack(txInfo, em)
		case "FollowUser":
			err = ci.followUser(txInfo, em)
		default:
			fmt.Println("no handler for ", action)
		}

		return err

	default:
		// fmt.Println("Unknown transaction type")
	}

	return nil
}

func (ci *CoreIndexer) createUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {

	var um GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &um)

	args := pgx.NamedArgs{
		"user_id":              em.EntityId,
		"handle_lc":            strings.ToLower(um.Data["handle"].(string)),
		"is_current":           true,
		"is_verified":          false,
		"created_at":           time.Now(),
		"updated_at":           time.Now(),
		"has_collectibles":     false,
		"txhash":               txInfo.txhash,
		"is_deactivated":       false,
		"is_available":         true,
		"is_storage_v2":        false,
		"allow_ai_attribution": false,
	}

	for k, v := range um.Data {
		if k == "events" {
			continue
		}
		args[k] = v
	}

	return ci.doInsert("users", args)
}

func (ci *CoreIndexer) createTrack(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {
	var meta GenericMetadata
	json.Unmarshal([]byte(em.Metadata), &meta)

	args := pgx.NamedArgs{
		"blockhash":                             txInfo.blockhash,
		"track_id":                              em.EntityId,
		"is_current":                            true,
		"is_delete":                             false,
		"owner_id":                              "@owner_id",
		"title":                                 "@title",
		"created_at":                            time.Now(),
		"updated_at":                            time.Now(),
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

func (ci *CoreIndexer) followUser(txInfo TxInfo, em *core_proto.ManageEntityLegacy) error {

	args := pgx.NamedArgs{
		"blockhash":        txInfo.blockhash,
		"blocknumber":      txInfo.blocknumber,
		"follower_user_id": em.UserId,
		"followee_user_id": em.EntityId,
		"is_current":       true,
		"is_delete":        false,
		"created_at":       time.Now(), // TODO
		"txhash":           txInfo.txhash,
		"slot":             500, // TODO
	}

	return ci.doInsert("follows", args)
}

type TxInfo struct {
	blockhash   string
	blocknumber int
	txhash      string
}

type GenericMetadata struct {
	CID  string         `json:"cid"`
	Data map[string]any `json:"data"`
}

func (ci *CoreIndexer) doInsert(tableName string, args pgx.NamedArgs) error {
	fields := []string{}
	placeholders := []string{}
	for field := range args {
		fields = append(fields, field)
		placeholders = append(placeholders, "@"+field)
	}
	stmt := fmt.Sprintf("insert into %s (%s) values (%s)",
		tableName,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := ci.pool.Exec(context.Background(), stmt, args)
	return err
}
