package indexer

import (
	"crypto/ecdsa"
	"encoding/base64"
	"log"

	corev1 "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	core_config "github.com/AudiusProject/audiusd/pkg/core/config"
	"github.com/AudiusProject/audiusd/pkg/core/server"
	"github.com/ethereum/go-ethereum/crypto"
)

func (ci *CoreIndexer) SetPubkeyForUser(userId int32, pubkey *ecdsa.PublicKey) {
	pubkeyBytes := crypto.FromECDSAPub(pubkey)
	pubkeyBase64 := base64.StdEncoding.EncodeToString(pubkeyBytes)
	log.Printf("userId: %d, pubkeyBase64: %s", userId, pubkeyBase64)
	// _, err := proc.writePool.Exec(context.Background(), `insert into user_pubkeys values ($1, $2) on conflict do nothing`, userId, pubkeyBase64)
	// if err != nil {
	// 	proc.logger.Warn("failed to set pubkey for user", zap.Error(err))
	// }
}

func (ci *CoreIndexer) createUser(em *corev1.ManageEntityLegacy) error {
	// TODO: need user_id from em tx
	// TODO: insert
	_, pubkey, err := server.RecoverPubkeyFromCoreTx(&core_config.Config{
		AcdcChainID:              ci.Config.AcdcChainID,
		AcdcEntityManagerAddress: ci.Config.AcdcEntityManagerAddress,
	}, em)
	if err != nil {
		return err
	}

	// TODO: check if user already exists

	ci.SetPubkeyForUser(int32(em.EntityId), pubkey)
	return nil
}
