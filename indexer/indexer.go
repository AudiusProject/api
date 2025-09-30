package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"connectrpc.com/connect"
	corev1 "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/AudiusProject/audiusd/pkg/sdk"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type CoreIndexer struct {
	pool    database.DbPool
	Config  config.Config
	logger  *zap.Logger
	closeCh chan struct{}
}

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

// TODO: open tx, during commit set block height, rollback etc.

func (ci *CoreIndexer) Start(ctx context.Context) error {
	sdk := sdk.NewAudiusdSDK(ci.Config.AudiusdURL)
	nodeInfo, err := sdk.Core.GetNodeInfo(context.Background(), connect.NewRequest(&corev1.GetNodeInfoRequest{}))
	if err != nil {
		return err
	}

	ci.logger.Info("Core indexer started at height", zap.Int64("height", nodeInfo.Msg.CurrentHeight))

	height := nodeInfo.Msg.CurrentHeight

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

		// channel timer prob better
		if block.Msg.Block.Height < 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		// TODO: Block also has a current chain height, to calc block diff
		err = ci.handleBlock(block.Msg.Block)
		if err != nil {
			log.Fatal(err)
		}

		height++
	}
}

func (ci *CoreIndexer) handleBlock(block *corev1.Block) error {
	for _, tx := range block.Transactions {
		if txData := tx.GetTransaction(); txData != nil {
			switch txData.GetTransaction().(type) {
			case *corev1.SignedTransaction_ManageEntity:
				em := txData.GetManageEntity()
				if em == nil {
					ci.logger.Error("ManageEntity transaction with empty data", zap.Any("tx", tx))
					continue
				}
				err := ci.handleManageEntity(em)
				if err != nil {
					ci.logger.Error("Error processing manage entity tx", zap.Error(err))
					continue
				}
			}
		}
	}
	return nil
}

func (ci *CoreIndexer) handleManageEntity(em *corev1.ManageEntityLegacy) error {
	operation := em.Action + em.EntityType
	switch operation {
	case "CreateUser":
		return ci.createUser(em)
	default:
		return nil
	}
}

func (ci *CoreIndexer) Close() {
	ci.pool.Close()
}
