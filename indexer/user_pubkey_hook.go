package indexer

import (
	"context"
	"encoding/base64"

	"api.audius.co/config"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	core_config "github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/server"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
)

// newUserPubkeyHook returns a pkg/etl post-hook that recovers the EIP-712
// public key from each User Create ManageEntity tx and writes it to
// user_pubkeys.
//
// Restores the pre-vendor `setPubkeyForUser` behavior that lived in
// indexer/index_user.go before ETL was vendored. The hook runs in the
// same DB transaction as the User Create handler (Params.DBTX), so the
// pubkey row commits atomically with the user row.
//
// `user_pubkeys` is read by two downstream paths:
//
//   - api/comms_pubkey.go (chat pubkey lookup)
//   - api/v1_users_sales_download.go (LEFT JOIN to attach buyer pubkey
//     onto sales-download receipts)
//
// The comms processor (api/comms/rpc_processor.go SetPubkeyForUser) also
// inserts into the same table when a user first sends a chat — that path
// continues to work in parallel. The hook here ensures the row lands at
// account-creation time, so buyers who buy something before ever sending
// a chat still have their pubkey on the sales-download join.
//
// Hook errors are logged but do NOT fail the parent User Create dispatch
// (see em.PostHook docs) — matches the upstream "log and continue"
// semantics. If recovery fails or the INSERT errors, the user row still
// commits and we'll get the pubkey later via the comms path.
func newUserPubkeyHook(cfg config.Config, logger *zap.Logger) em.PostHook {
	coreCfg := &core_config.Config{
		AcdcChainID:              cfg.AudiusdChainID,
		AcdcEntityManagerAddress: cfg.AudiusdEntityManagerAddress,
	}
	hookLogger := logger.Named("UserPubkeyHook")

	return func(ctx context.Context, params *em.Params) error {
		tx := params.TX
		if tx == nil {
			return nil // defensive — proto should always be set on dispatch
		}
		return recoverAndStorePubkey(ctx, hookLogger, coreCfg, tx, params)
	}
}

func recoverAndStorePubkey(
	ctx context.Context,
	logger *zap.Logger,
	coreCfg *core_config.Config,
	tx *corev1.ManageEntityLegacy,
	params *em.Params,
) error {
	_, pubkey, err := server.RecoverPubkeyFromCoreTx(coreCfg, tx)
	if err != nil {
		// Recovery failure on a freshly indexed CreateUser is unexpected;
		// log so we can spot it, but don't propagate.
		logger.Warn("failed to recover pubkey from CreateUser tx",
			zap.Int64("user_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	pubkeyBase64 := base64.StdEncoding.EncodeToString(crypto.FromECDSAPub(pubkey))

	if _, err := params.DBTX.Exec(ctx,
		`INSERT INTO user_pubkeys (user_id, pubkey_base64) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		params.EntityID, pubkeyBase64,
	); err != nil {
		logger.Warn("failed to insert user_pubkeys row",
			zap.Int64("user_id", params.EntityID),
			zap.Error(err))
		return nil
	}

	logger.Debug("indexed user pubkey",
		zap.Int64("user_id", params.EntityID),
		zap.String("pubkey_base64", pubkeyBase64))
	return nil
}
