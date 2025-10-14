package dbc

import (
	"context"
	"fmt"
	"strings"

	"api.audius.co/database"
	"api.audius.co/solana/indexer/common"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func processDbcInstruction(
	ctx context.Context,
	db database.DBTX,
	rpcClient common.RpcClient,
	slot uint64,
	tx *solana.Transaction,
	instructionIndex int,
	instruction solana.CompiledInstruction,
	signature string,
	instLogger *zap.Logger,
) error {
	accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
	if err != nil {
		return fmt.Errorf("error resolving instruction accounts %d: %w", instructionIndex, err)
	}

	inst, err := meteora_dbc.DecodeInstruction(accounts, []byte(instruction.Data))
	if err != nil {
		// Ignore unknown instruction types.
		// Not all DBC instruction types are implemented yet.
		// See: solana/spl/programs/meteora_dbc/instruction.go
		// See: https://github.com/gagliardetto/binary/blob/v0.8.0/variant.go#L315
		if strings.Contains(err.Error(), "no known type for type") {
			return nil // ignore unknown instruction types
		}
		return fmt.Errorf("error decoding meteora_dbc instruction %d: %w", instructionIndex, err)
	}

	switch inst.TypeID {
	case meteora_dbc.InstructionImplDef.TypeID(meteora_dbc.Instruction_MigrationDammV2):
		{
			if migrationInst, ok := inst.Impl.(*meteora_dbc.MigrationDammV2); ok {
				err := insertDbcMigration(ctx, db, dbcMigrationRow{
					signature:                signature,
					instructionIndex:         instructionIndex,
					slot:                     slot,
					dbcPool:                  migrationInst.GetVirtualPool().PublicKey.String(),
					migrationMetadata:        migrationInst.GetMigrationMetadata().PublicKey.String(),
					config:                   migrationInst.GetConfig().PublicKey.String(),
					dbcPoolAuthority:         migrationInst.GetPoolAuthority().PublicKey.String(),
					dammV2Pool:               migrationInst.GetPool().PublicKey.String(),
					firstPositionNftMint:     migrationInst.GetFirstPositionNftMint().PublicKey.String(),
					firstPositionNftAccount:  migrationInst.GetFirstPositionNftAccount().PublicKey.String(),
					firstPosition:            migrationInst.GetFirstPosition().PublicKey.String(),
					secondPositionNftMint:    migrationInst.GetSecondPositionNftMint().PublicKey.String(),
					secondPositionNftAccount: migrationInst.GetSecondPositionNftAccount().PublicKey.String(),
					secondPosition:           migrationInst.GetSecondPosition().PublicKey.String(),
					dammPoolAuthority:        migrationInst.GetPoolAuthority().PublicKey.String(),
					baseMint:                 migrationInst.GetBaseMint().PublicKey.String(),
					quoteMint:                migrationInst.GetQuoteMint().PublicKey.String(),
				})
				if err != nil {
					return fmt.Errorf("failed to insert dbc migration at instruction %d: %w", instructionIndex, err)
				}
				instLogger.Info("dbc migrationDammV2",
					zap.String("mint", migrationInst.GetBaseMint().PublicKey.String()),
					zap.String("dbcPool", migrationInst.GetVirtualPool().PublicKey.String()),
					zap.String("dammV2Pool", migrationInst.GetPool().PublicKey.String()),
				)

				// Also index the pool and positions

				// var dammPool meteora_damm_v2.Pool
				// err = common.WithRetries(func() error {
				// 	return rpcClient.GetAccountDataBorshInto(ctx, migrationInst.GetPool().PublicKey, &dammPool)
				// }, 5, time.Second*1)
				// if err != nil {
				// 	return fmt.Errorf("failed to get damm v2 pool account data after retries: %w", err)
				// } else {
				// 	err = upsertDammV2Pool(ctx, db, slot, migrationInst.GetPool().PublicKey, &dammPool)
				// 	if err != nil {
				// 		return fmt.Errorf("failed to upsert damm v2 pool: %w", err)
				// 	}
				// }

				// var firstPosition meteora_damm_v2.PositionState
				// err = common.WithRetries(func() error {
				// 	return rpcClient.GetAccountDataBorshInto(ctx, migrationInst.GetFirstPosition().PublicKey, &firstPosition)
				// }, 5, time.Second*1)
				// if err != nil {
				// 	return fmt.Errorf("failed to get first damm v2 position account data: %w", err)
				// } else {
				// 	err = upsertDammV2Position(ctx, db, slot, migrationInst.GetFirstPosition().PublicKey, &firstPosition)
				// 	if err != nil {
				// 		return fmt.Errorf("failed to upsert first damm v2 position: %w", err)
				// 	}
				// }

				// var secondPosition meteora_damm_v2.PositionState
				// err = common.WithRetries(func() error {
				// 	return rpcClient.GetAccountDataBorshInto(ctx, migrationInst.GetSecondPosition().PublicKey, &secondPosition)
				// }, 5, time.Second*1)
				// if err != nil {
				// 	return fmt.Errorf("failed to get second damm v2 position account data: %w", err)
				// } else {
				// 	err = upsertDammV2Position(ctx, db, slot, migrationInst.GetSecondPosition().PublicKey, &secondPosition)
				// 	if err != nil {
				// 		return fmt.Errorf("failed to upsert second damm v2 position: %w", err)
				// 	}
				// }
			}
		}
	}
	return nil
}

type dbcMigrationRow struct {
	signature                string
	instructionIndex         int
	slot                     uint64
	dbcPool                  string
	migrationMetadata        string
	config                   string
	dbcPoolAuthority         string
	dammV2Pool               string
	firstPositionNftMint     string
	firstPositionNftAccount  string
	firstPosition            string
	secondPositionNftMint    string
	secondPositionNftAccount string
	secondPosition           string
	dammPoolAuthority        string
	baseMint                 string
	quoteMint                string
}

func insertDbcMigration(ctx context.Context, db database.DBTX, row dbcMigrationRow) error {
	sql := `
	INSERT INTO sol_meteora_dbc_migrations (
		signature,
		instruction_index,
		slot,
		dbc_pool,
		migration_metadata,
		config,
		dbc_pool_authority,
		damm_v2_pool,
		first_position_nft_mint,
		first_position_nft_account,
		first_position,
		second_position_nft_mint,
		second_position_nft_account,
		second_position,
		damm_pool_authority,
		base_mint,
		quote_mint,
		created_at,
		updated_at
	) VALUES (
		@signature,
		@instructionIndex,
		@slot,
		@dbcPool,
		@migrationMetadata,
		@config,
		@dbcPoolAuthority,
		@dammV2Pool,
		@firstPositionNftMint,
		@firstPositionNftAccount,
		@firstPosition,
		@secondPositionNftMint,
		@secondPositionNftAccount,
		@secondPosition,
		@dammPoolAuthority,
		@baseMint,
		@quoteMint,
		NOW(),
		NOW()
	)
	ON CONFLICT DO NOTHING
	`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"signature":                row.signature,
		"instructionIndex":         row.instructionIndex,
		"slot":                     row.slot,
		"dbcPool":                  row.dbcPool,
		"migrationMetadata":        row.migrationMetadata,
		"config":                   row.config,
		"dbcPoolAuthority":         row.dbcPoolAuthority,
		"dammV2Pool":               row.dammV2Pool,
		"firstPositionNftMint":     row.firstPositionNftMint,
		"firstPositionNftAccount":  row.firstPositionNftAccount,
		"firstPosition":            row.firstPosition,
		"secondPositionNftMint":    row.secondPositionNftMint,
		"secondPositionNftAccount": row.secondPositionNftAccount,
		"secondPosition":           row.secondPosition,
		"dammPoolAuthority":        row.dammPoolAuthority,
		"baseMint":                 row.baseMint,
		"quoteMint":                row.quoteMint,
	})
	return err
}
