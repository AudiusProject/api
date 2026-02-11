package api

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"api.audius.co/api/dbv1"
	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	coreconfig "github.com/OpenAudio/go-openaudio/pkg/core/config"
	coreserver "github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) sendTransactionWithSigner(manageEntityTx *corev1.ManageEntityLegacy, signer *ecdsa.PrivateKey) (*connect.Response[corev1.SendTransactionResponse], error) {
	coreConfig := coreconfig.Config{
		AcdcEntityManagerAddress: app.config.AudiusdEntityManagerAddress,
		AcdcChainID:              app.config.AudiusdChainID,
	}

	err := coreserver.SignManageEntity(&coreConfig, manageEntityTx, signer)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	sendReq := &corev1.SendTransactionRequest{
		Transaction: &corev1.SignedTransaction{
			Transaction: &corev1.SignedTransaction_ManageEntity{
				ManageEntity: manageEntityTx,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := app.openAudioSDK.Core.SendTransaction(ctx, connect.NewRequest(sendReq))
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	err = app.pollForBlockHash(ctx, response.Msg.Transaction.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to poll for transaction hash: %w", err)
	}

	return response, nil
}

func (app *ApiServer) pollForBlockHash(ctx context.Context, blockHash string) error {
	start := time.Now()
	defer func() {
		slog.Debug("Polled for block hash", "blockHash", blockHash, "duration", time.Since(start).String())
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	subCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	exists, err := checkBlockHashExists(subCtx, app.pool, blockHash)
	if err != nil {
		return fmt.Errorf("failed to check block hash existence: %w", err)
	}
	if exists {
		return nil
	}

	for {
		select {
		case <-subCtx.Done():
			return fmt.Errorf("context cancelled while polling for block hash: %w", ctx.Err())
		case <-ticker.C:
			exists, err := checkBlockHashExists(subCtx, app.pool, blockHash)
			if err != nil {
				return fmt.Errorf("failed to check block hash existence: %w", err)
			}
			if exists {
				return nil
			}
		}
	}
}

func checkBlockHashExists(ctx context.Context, pool *dbv1.DBPools, blockHash string) (bool, error) {
	sql := `SELECT EXISTS (
		SELECT 1 FROM core_indexed_blocks WHERE blockhash = $1
	);`
	row := pool.QueryRow(ctx, sql, blockHash)
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}
