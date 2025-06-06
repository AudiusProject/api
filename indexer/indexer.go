package indexer

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"connectrpc.com/connect"
	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/AudiusProject/audiusd/pkg/sdk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CoreIndexer struct {
	ctx  context.Context
	pool *pgxpool.Pool
	auds *sdk.AudiusdSDK
}

type CoreIndexerConfig struct {
	AudiusdURL string
	DbUrl      string
}

func NewIndexer(config CoreIndexerConfig) (*CoreIndexer, error) {
	bg := context.Background()
	pool, err := pgxpool.New(bg, config.DbUrl)
	if err != nil {
		return nil, err
	}

	auds := sdk.NewAudiusdSDK(config.AudiusdURL)

	ci := &CoreIndexer{
		bg,
		pool,
		auds,
	}

	return ci, nil
}

func (ci *CoreIndexer) handleTx(tx *core_proto.Transaction) error {
	signedTx := tx.GetTransaction()
	// tx timestamp is the block timestamp
	blockTime := tx.GetTimestamp()
	txInfo := TxInfo{
		txhash:      tx.Hash,
		blockhash:   "todo",
		blocknumber: 123,
		timestamp:   blockTime.AsTime(),
	}

	switch signedTx.GetTransaction().(type) {
	case *core_proto.SignedTransaction_Plays:
		// play := signedTx.GetPlays()
		// fmt.Println("PLAY", play)

	case *core_proto.SignedTransaction_ManageEntity:
		em := signedTx.GetManageEntity()
		action := em.Action + em.EntityType
		var err error

		// TODO: verify signature
		// TODO: and that em.Signer is authorized for em.UserId

		switch action {
		case "CreateUser":
			err = ci.createUser(txInfo, em)
		case "UpdateUser":
			err = ci.updateUser(txInfo, em)

		case "CreateTrack":
			err = ci.createTrack(txInfo, em)
		case "UpdateTrack":
			err = ci.updateTrack(txInfo, em)
		case "DeleteTrack":
			err = ci.deleteTrack(txInfo, em)
		case "DownloadTrack":
			err = ci.downloadTrack(txInfo, em)

		case "CreatePlaylist":
			err = ci.createPlaylist(txInfo, em)
		case "UpdatePlaylist":
			err = ci.updatePlaylist(txInfo, em)

		case "CreateComment":
			err = ci.createComment(txInfo, em)

		case "FollowUser":
			err = ci.followUser(txInfo, em)
		case "UnfollowUser":
			err = ci.unfollowUser(txInfo, em)
		case "RepostPlaylist", "RepostTrack":
			err = ci.repost(txInfo, em)
		case "SaveTrack", "SavePlaylist":
			err = ci.favorite(txInfo, em)
		case "UnsaveTrack", "UnsavePlaylist":
			err = ci.unfavorite(txInfo, em)
		case "ViewNotification":
			err = ci.viewNotification(txInfo, em)
		default:
			fmt.Println("no handler for ", action)
		}

		return err

	default:
		// fmt.Println("Unknown transaction type")
	}

	return nil
}

type TxInfo struct {
	blockhash   string
	blocknumber int
	txhash      string
	timestamp   time.Time
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

	_, err := ci.pool.Exec(ci.ctx, stmt, args)
	return err
}

func (ci *CoreIndexer) doUpdate(tableName string, args pgx.NamedArgs, where pgx.NamedArgs) error {
	fields := []string{}
	for field := range args {
		fields = append(fields, fmt.Sprintf("%s = @%s", field, field))
	}

	wheres := []string{}
	for field := range where {
		wheres = append(wheres, fmt.Sprintf("%s = @%s", field, field))
	}

	stmt := fmt.Sprintf("update %s set %s where %s",
		tableName,
		strings.Join(fields, ", "),
		strings.Join(wheres, " AND "),
	)

	maps.Copy(args, where)

	// fmt.Println(stmt, args)

	_, err := ci.pool.Exec(ci.ctx, stmt, args)
	return err
}

func (ci *CoreIndexer) Start(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	blockNum := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			nextBlock := blockNum + 1
			block, err := ci.auds.Core.GetBlock(ci.ctx, connect.NewRequest(&core_proto.GetBlockRequest{
				Height: int64(nextBlock),
			}))
			if err != nil {
				return err
			}

			for _, tx := range block.Msg.Block.Transactions {
				err := ci.handleTx(tx)
				if err != nil {
					return err
				}
			}

			blockNum++
		}
	}
}
