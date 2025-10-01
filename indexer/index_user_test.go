package indexer

import (
	"testing"

	"api.audius.co/api/testdata"
	"api.audius.co/config"
	"api.audius.co/database"
	corev1 "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	core_config "github.com/AudiusProject/audiusd/pkg/core/config"
	core_server "github.com/AudiusProject/audiusd/pkg/core/server"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// dummy pkeys generated from ganache "test test...junk" seed
var user1WalletKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestIndexCreateUserPubkey(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	defer pool.Close()
	logger := zap.NewNop()

	config := config.Config{
		WriteDbUrl:                  pool.Config().ConnString(),
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}

	ci := NewIndexer(config)
	defer ci.Close()

	wallet := testdata.CreateTestWallet(t, user1WalletKey)

	em := &corev1.ManageEntityLegacy{
		UserId:     1,
		EntityId:   1,
		EntityType: "User",
		Action:     "Create",
		Metadata:   "{}",
		Nonce:      "1",
	}

	core_server.SignManageEntity(&core_config.Config{
		AcdcChainID:              config.AudiusdChainID,
		AcdcEntityManagerAddress: config.AudiusdEntityManagerAddress,
	}, em, wallet.PrivateKey)

	err := ci.handleManageEntity(pool, logger, em)
	assert.NoError(t, err)

	var pubkey string
	err = pool.QueryRow(t.Context(), `
		SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = 1
	`).Scan(&pubkey)
	assert.NoError(t, err)
	assert.Equal(t, pubkey, "BIMYU1tUEF1Keq5gwI/EX5aHGBtP38YlvRp1P6c5f+11NUfxHKhpZkby86ywjjEBavrCPmMMXRH1n2H+9XsNKqU=")
}

func TestIndexCreateUserExistingPubkey(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	defer pool.Close()
	logger := zap.NewNop()

	database.Seed(pool, database.FixtureMap{
		"user_pubkeys": {
			{
				"user_id":       1,
				"pubkey_base64": "existing_pubkey",
			},
		},
	})

	config := config.Config{
		WriteDbUrl:                  pool.Config().ConnString(),
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}

	ci := NewIndexer(config)
	defer ci.Close()

	wallet := testdata.CreateTestWallet(t, user1WalletKey)

	em := &corev1.ManageEntityLegacy{
		UserId:     1,
		EntityId:   1,
		EntityType: "User",
		Action:     "Create",
		Metadata:   "{}",
		Nonce:      "1",
	}

	core_server.SignManageEntity(&core_config.Config{
		AcdcChainID:              config.AudiusdChainID,
		AcdcEntityManagerAddress: config.AudiusdEntityManagerAddress,
	}, em, wallet.PrivateKey)

	err := ci.handleManageEntity(pool, logger, em)
	assert.NoError(t, err)

	var pubkey string
	err = pool.QueryRow(t.Context(), `
		SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = 1
	`).Scan(&pubkey)
	assert.NoError(t, err)
	assert.Equal(t, pubkey, "existing_pubkey")
}
