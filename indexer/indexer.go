package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

	"api.audius.co/config"
	dbv1 "api.audius.co/database"
	"api.audius.co/logging"
	"connectrpc.com/connect"
	corev1 "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/AudiusProject/audiusd/pkg/sdk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type CoreIndexer struct {
	pool    dbv1.DbPool
	Config  config.Config
	logger  *zap.Logger
	closeCh chan struct{}
}

const (
	coreIndexerCheckpointName = "api_core_indexer_last_height"
)

func NewIndexer(config config.Config) *CoreIndexer {

	connConfig, err := pgxpool.ParseConfig(config.WriteDbUrl)
	if err != nil {
		panic(fmt.Errorf("error parsing database URL: %w", err))
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	ci := &CoreIndexer{
		pool:   pool,
		Config: config,
		logger: logging.NewZapLogger(config).
			Named("CoreIndexer"),
	}

	return ci
}

func (ci *CoreIndexer) Start(ctx context.Context) error {
	sdk := sdk.NewAudiusdSDK(ci.Config.AudiusdURL)

	var height int64
	err := ci.pool.QueryRow(context.Background(), `select last_checkpoint from indexing_checkpoints where tablename = $1`, coreIndexerCheckpointName).Scan(&height)
	if err != nil {
		if err == pgx.ErrNoRows {
			nodeInfo, err := sdk.Core.GetNodeInfo(context.Background(), connect.NewRequest(&corev1.GetNodeInfoRequest{}))
			if err != nil {
				return err
			}
			height = nodeInfo.Msg.CurrentHeight
		} else {
			return err
		}
	} else {
		// If we have a checkpoint, we need to start at the next block
		height++
	}

	ci.logger.Info("Core indexer started", zap.Int64("blockHeight", height))

	for {
		select {
		case <-ctx.Done():
			ci.logger.Info("Shutting down core indexer")
			return nil
		default:
		}
		block, err := sdk.Core.GetBlock(context.Background(), connect.NewRequest(&corev1.GetBlockRequest{
			Height: height,
		}))
		if err != nil {
			log.Fatal(err)
		}

		if block.Msg.Block.Height < 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		err = ci.handleBlock(block.Msg.Block)
		if err != nil {
			log.Fatal(err)
		}

		height++
	}
}

func (ci *CoreIndexer) handleBlock(block *corev1.Block) error {
	dbTx, err := ci.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer dbTx.Rollback(context.Background())

	for _, tx := range block.Transactions {
		if txData := tx.GetTransaction(); txData != nil {
			logger := ci.logger.With(zap.String("txHash", tx.GetHash()), zap.Int64("blockHeight", block.Height))
			switch txData.GetTransaction().(type) {
			case *corev1.SignedTransaction_ManageEntity:
				em := txData.GetManageEntity()
				if em == nil {
					ci.logger.Error("ManageEntity transaction with empty data", zap.Any("tx", tx))
					continue
				}
				err := ci.handleManageEntity(dbTx, logger, em)
				if err != nil {
					logger.Error("Error processing manage entity tx", zap.Error(err))
					continue
				}
			}
		}
	}
	_, err = dbTx.Exec(context.Background(), `
	insert into indexing_checkpoints values ($1, $2)
	on conflict (tablename) do update set last_checkpoint = excluded.last_checkpoint
	`, coreIndexerCheckpointName, block.Height)

	if err != nil {
		return err
	}

	err = dbTx.Commit(context.Background())
	if err != nil {
		return err
	}

	ci.logger.Debug("Indexed block", zap.Int64("blockHeight", block.Height))

	return nil
}

func (ci *CoreIndexer) handleManageEntity(dbTx dbv1.DBTX, logger *zap.Logger, em *corev1.ManageEntityLegacy) error {
	operation := em.Action + em.EntityType
	switch operation {
	case "CreateUser":
		return ci.createUser(dbTx, logger, em)
	default:
		return nil
	}
}

func (ci *CoreIndexer) Close() {
	ci.pool.Close()
}
