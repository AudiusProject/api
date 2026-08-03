package indexer

import (
	"context"
	"testing"

	"api.audius.co/api/testdata"
	"api.audius.co/config"
	"api.audius.co/database"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	core_config "github.com/OpenAudio/go-openaudio/pkg/core/config"
	core_server "github.com/OpenAudio/go-openaudio/pkg/core/server"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Dummy ganache "test test...junk" seed key — same one the pre-vendor
// indexer test used, so the recovered pubkey below is identical to what
// the old setPubkeyForUser path produced.
const user1WalletKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// Expected pubkey recovered from the EIP-712 signature of the
// ManageEntity tx below, base64-encoded with crypto.FromECDSAPub.
// Matches the value asserted by the pre-vendor TestIndexCreateUserPubkey.
const expectedPubkey = "BIMYU1tUEF1Keq5gwI/EX5aHGBtP38YlvRp1P6c5f+11NUfxHKhpZkby86ywjjEBavrCPmMMXRH1n2H+9XsNKqU="

// signedCreateUserTx builds a signed User/Create ManageEntity proto using
// the dev EM chain config + test wallet. Returns the proto plus the
// matching core config the hook should be constructed with.
func signedCreateUserTx(t *testing.T) (*corev1.ManageEntityLegacy, *core_config.Config) {
	t.Helper()
	wallet := testdata.CreateTestWallet(t, user1WalletKey)

	tx := &corev1.ManageEntityLegacy{
		UserId:     1,
		EntityId:   1,
		EntityType: "User",
		Action:     "Create",
		Metadata:   "{}",
		Nonce:      "1",
	}
	coreCfg := &core_config.Config{
		AcdcChainID:              core_config.DevAcdcChainID,
		AcdcEntityManagerAddress: core_config.DevAcdcAddress,
	}
	if err := core_server.SignManageEntity(coreCfg, tx, wallet.PrivateKey); err != nil {
		t.Fatalf("SignManageEntity: %v", err)
	}
	return tx, coreCfg
}

// TestUserPubkeyHook_InsertsRow verifies the hook recovers the EIP-712
// pubkey from a CreateUser tx and inserts the matching base64 row into
// user_pubkeys. This is the behavior the pre-vendor setPubkeyForUser
// path covered; restoring it via the upstream pkg/etl post-create hook
// is the whole point of this file.
func TestUserPubkeyHook_InsertsRow(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	tx, _ := signedCreateUserTx(t)

	cfg := config.Config{
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}
	hook := newUserPubkeyHook(cfg, logger)
	err := hook(context.Background(), &em.Params{
		TX:         tx,
		EntityID:   1,
		EntityType: em.EntityTypeUser,
		Action:     em.ActionCreate,
		DBTX:       pool,
	})
	assert.NoError(t, err)

	var pubkey string
	err = pool.QueryRow(t.Context(),
		`SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = 1`,
	).Scan(&pubkey)
	assert.NoError(t, err)
	assert.Equal(t, expectedPubkey, pubkey)
}

// TestUserPubkeyHook_ExistingRowPreserved verifies the ON CONFLICT DO
// NOTHING semantics: if a user_pubkeys row already exists (typically
// from the comms `SetPubkeyForUser` path firing before the indexer
// catches up), the hook must not overwrite it. The chat path's value
// is canonical for that user — the indexed-from-tx value is just a
// fallback for the sales-download join.
func TestUserPubkeyHook_ExistingRowPreserved(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	database.Seed(pool, database.FixtureMap{
		"user_pubkeys": {
			{"user_id": 1, "pubkey_base64": "existing_pubkey"},
		},
	})

	tx, _ := signedCreateUserTx(t)

	cfg := config.Config{
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}
	hook := newUserPubkeyHook(cfg, logger)
	err := hook(context.Background(), &em.Params{
		TX:         tx,
		EntityID:   1,
		EntityType: em.EntityTypeUser,
		Action:     em.ActionCreate,
		DBTX:       pool,
	})
	assert.NoError(t, err)

	var pubkey string
	err = pool.QueryRow(t.Context(),
		`SELECT pubkey_base64 FROM user_pubkeys WHERE user_id = 1`,
	).Scan(&pubkey)
	assert.NoError(t, err)
	assert.Equal(t, "existing_pubkey", pubkey)
}

// TestUserPubkeyHook_NilProto verifies the defensive nil-check: if a
// caller somehow dispatches a User/Create with no proto attached
// (shouldn't happen — applyPendingPostHooks runs after the handler
// which itself receives the proto via Params.TX), the hook returns
// nil without panicking and writes nothing.
func TestUserPubkeyHook_NilProto(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	cfg := config.Config{
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}
	hook := newUserPubkeyHook(cfg, logger)
	err := hook(context.Background(), &em.Params{
		TX:         nil,
		EntityID:   1,
		EntityType: em.EntityTypeUser,
		Action:     em.ActionCreate,
		DBTX:       pool,
	})
	assert.NoError(t, err)

	var count int
	err = pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM user_pubkeys WHERE user_id = 1`,
	).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestUserPubkeyHook_BadSignatureLogsAndContinues verifies that pubkey
// recovery failure (e.g. signature tampered with after signing) is
// swallowed — the hook logs a warning and returns nil. This matches
// the upstream PostHook "log and continue" contract: a failed pubkey
// recovery must not roll back the user row.
func TestUserPubkeyHook_BadSignatureLogsAndContinues(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_indexer")
	logger := zap.NewNop()

	tx, _ := signedCreateUserTx(t)
	// Corrupt the signature — recovery will now fail.
	tx.Signature = "0xdeadbeef"

	cfg := config.Config{
		AudiusdChainID:              core_config.DevAcdcChainID,
		AudiusdEntityManagerAddress: core_config.DevAcdcAddress,
	}
	hook := newUserPubkeyHook(cfg, logger)
	err := hook(context.Background(), &em.Params{
		TX:         tx,
		EntityID:   1,
		EntityType: em.EntityTypeUser,
		Action:     em.ActionCreate,
		DBTX:       pool,
	})
	assert.NoError(t, err)

	var count int
	err = pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM user_pubkeys WHERE user_id = 1`,
	).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
