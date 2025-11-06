package indexer

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"

	dbv1 "api.audius.co/database"
	"go.uber.org/zap"

	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	core_config "github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/ethereum/go-ethereum/crypto"
)

func (ci *CoreIndexer) setPubkeyForUser(dbTx dbv1.DBTX, logger *zap.Logger, userId int32, pubkey *ecdsa.PublicKey) {
	pubkeyBytes := crypto.FromECDSAPub(pubkey)
	pubkeyBase64 := base64.StdEncoding.EncodeToString(pubkeyBytes)

	logger.Info("CreateUser, setting pubkey", zap.Int32("userId", userId), zap.String("pubkeyBase64", pubkeyBase64))

	_, err := dbTx.Exec(context.Background(), `insert into user_pubkeys values ($1, $2) on conflict do nothing`, userId, pubkeyBase64)

	if err != nil {
		logger.Warn("failed to set pubkey for user", zap.Int32("userId", userId), zap.String("pubkeyBase64", pubkeyBase64), zap.Error(err))
	}
}

func (ci *CoreIndexer) createUser(dbTx dbv1.DBTX, logger *zap.Logger, em *corev1.ManageEntityLegacy) error {
	_, pubkey, err := server.RecoverPubkeyFromCoreTx(&core_config.Config{
		AcdcChainID:              ci.Config.AudiusdChainID,
		AcdcEntityManagerAddress: ci.Config.AudiusdEntityManagerAddress,
	}, em)
	if err != nil {
		return err
	}

	ci.setPubkeyForUser(dbTx, logger, int32(em.EntityId), pubkey)
	return nil
}
