package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"api.audius.co/config"
	"api.audius.co/solana/indexer/damm_v2"
	"api.audius.co/solana/indexer/dbc"
	"api.audius.co/solana/indexer/locker"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	"github.com/AlecAivazis/survey/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

var coinCmd = &cobra.Command{
	Use:   "coin [mint]",
	Short: "Refresh all data related to a coin",
	Long:  `Fetches all related content to a coin via Solana RPC to backfill into the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rpcEndpoint, err := cmd.Flags().GetString("rpc")
		if err != nil {
			return fmt.Errorf("failed to get rpc flag: %w", err)
		}
		rpcClient := rpc.New(rpcEndpoint)

		updates := make([]string, 0)
		ctx := cmd.Context()
		mint := solana.MustPublicKeyFromBase58(args[0])
		fmt.Println("Refreshing coin mint:", mint)

		skip, err := cmd.Flags().GetBool("skip")
		if err != nil {
			return fmt.Errorf("failed to get skip flag: %w", err)
		}

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return fmt.Errorf("failed to get all flag: %w", err)
		}

		dbcPools := make([]string, 0)
		dbcPool, err := cmd.Flags().GetString("dbc-pool")
		if err != nil {
			return fmt.Errorf("failed to get dbc-pool flag: %w", err)
		}
		if dbcPool != "" {
			dbcPools = append(dbcPools, strings.Split(dbcPool, ",")...)
		} else if !skip {
			pools, err := findDbcPoolsForMint(ctx, rpcClient, mint)
			if err != nil {
				return fmt.Errorf("failed to find DBC pools for mint %s: %w", mint, err)
			}
			if all {
				dbcPools = append(dbcPools, pools...)
			} else {
				dbcPools = append(dbcPools, selectAccounts(pools)...)
			}
		}

		dbcConfigs := make([]string, 0)
		dbcConfig, err := cmd.Flags().GetString("dbc-config")
		if err != nil {
			return fmt.Errorf("failed to get dbc-config flag: %w", err)
		}
		if dbcConfig != "" {
			dbcConfigs = append(dbcConfigs, dbcConfig)
		} else if !skip {
			for _, pool := range dbcPools {
				var data meteora_dbc.Pool
				err := rpcClient.GetAccountDataBorshInto(ctx, solana.MustPublicKeyFromBase58(pool), &data)
				if err != nil {
					return fmt.Errorf("failed to find DBC configs for pool %s: %w", pool, err)
				}
				dbcConfigs = append(dbcConfigs, data.Config.String())
			}
		}

		dammV2Pools := make([]string, 0)
		dammV2Pool, err := cmd.Flags().GetString("damm-v2-pool")
		if err != nil {
			return fmt.Errorf("failed to get damm-v2-pool flag: %w", err)
		}

		if dammV2Pool != "" {
			dammV2Pools = append(dammV2Pools, strings.Split(dammV2Pool, ",")...)
		} else if !skip {
			pools, err := findDammV2PoolsForMint(ctx, rpcClient, mint)
			if err != nil {
				return fmt.Errorf("failed to find Damm V2 pools for mint %s: %w", mint, err)
			}
			if all {
				dammV2Pools = append(dammV2Pools, pools...)
			} else {
				dammV2Pools = append(dammV2Pools, selectAccounts(pools)...)
			}
		}

		dammV2Positions := make([]string, 0)
		dammV2Position, err := cmd.Flags().GetString("damm-v2-position")
		if err != nil {
			return fmt.Errorf("failed to get damm-v2-position flag: %w", err)
		}
		if dammV2Position != "" {
			dammV2Positions = append(dammV2Positions, strings.Split(dammV2Position, ",")...)
		} else if !skip {
			for _, pool := range dammV2Pools {
				positions, err := findDammV2PositionsForPool(ctx, rpcClient, solana.MustPublicKeyFromBase58(pool))
				if err != nil {
					return fmt.Errorf("failed to find Damm V2 positions for pool %s: %w", pool, err)
				}
				if all {
					dammV2Positions = append(dammV2Positions, positions...)
				} else {
					dammV2Positions = append(dammV2Positions, selectAccounts(positions)...)
				}
			}
		}

		dammV2Inits := make(map[string]string)
		for _, position := range dammV2Positions {
			migrationOrInit, err := findMigrationOrInitForPosition(ctx, rpcClient, solana.MustPublicKeyFromBase58(position))
			if err != nil {
				return fmt.Errorf("failed to find migration or init for position %s: %w", position, err)
			}
			if migrationOrInit != nil {
				fmt.Printf("Found migration or init tx %s for position %s\n", *migrationOrInit, position)
				dammV2Inits[position] = *migrationOrInit
			}
		}

		lockers := make([]string, 0)
		lock, err := cmd.Flags().GetString("locker")
		if err != nil {
			return fmt.Errorf("failed to get locker flag: %w", err)
		}
		if lock != "" {
			lockers = append(lockers, strings.Split(lock, ",")...)
		} else if !skip {
			temp, err := findLockersForMintAndDbcPools(ctx, rpcClient, mint, dbcPools)
			if err != nil {
				return fmt.Errorf("failed to find lockers for mint %s and DBC pools %v: %w", mint, dbcPools, err)
			}
			if all {
				lockers = append(lockers, temp...)
			} else {
				lockers = append(lockers, selectAccounts(temp)...)
			}
		}

		updateCoin, err := cmd.Flags().GetBool("update-coin")
		if err != nil {
			return fmt.Errorf("failed to get update-coin flag: %w", err)
		}
		if updateCoin {
			if len(dammV2Pools) != 1 {
				return fmt.Errorf("update-coin flag requires exactly one Damm V2 pool selected")
			}
			updates = append(updates, fmt.Sprintf("UPDATE artist_coins SET damm_v2_pool = '%s' WHERE mint = '%s';\n", dammV2Pools[0], mint.String()))
		}

		fmt.Printf("\n\033[36m=== Summary ===\033[0m\n\n")

		if updateCoin {
			fmt.Println("Updating coin Damm V2 pool:\t", dammV2Pools[0])
		}

		for _, pool := range dbcPools {
			fmt.Println("Refreshing DBC pool:\t", pool)
			memo := fmt.Sprintf("Coin %s refresh DBC pool %s", mint, pool)
			sql, err := refreshAccount(ctx, rpcClient, dbc.NAME, pool, memo, mint.String())
			if err != nil {
				return fmt.Errorf("failed to refresh DBC pool: %w", err)
			}
			updates = append(updates, sql)
		}

		for _, config := range dbcConfigs {
			fmt.Println("Refreshing DBC config:\t", config)
			memo := fmt.Sprintf("Coin %s refresh DBC config %s", mint, config)
			sql, err := refreshAccount(ctx, rpcClient, dbc.NAME, config, memo, "dbc_config")
			if err != nil {
				return fmt.Errorf("failed to refresh DBC config: %w", err)
			}
			updates = append(updates, sql)
		}

		for _, pool := range dammV2Pools {
			fmt.Println("Refreshing Damm V2 pool:\t", pool)
			memo := fmt.Sprintf("Coin %s refresh Damm V2 pool %s", mint, pool)
			sql, err := refreshAccount(ctx, rpcClient, damm_v2.NAME, pool, memo, damm_v2.NAME)
			if err != nil {
				return fmt.Errorf("failed to refresh Damm V2 pool: %w", err)
			}
			updates = append(updates, sql)
		}

		for _, position := range dammV2Positions {
			fmt.Println("Refreshing Damm V2 position:\t", position)
			memo := fmt.Sprintf("Coin %s refresh Damm V2 position %s", mint, position)
			sql, err := refreshAccount(ctx, rpcClient, damm_v2.NAME, position, memo, position)
			if err != nil {
				return fmt.Errorf("failed to refresh Damm V2 position: %w", err)
			}
			updates = append(updates, sql)
		}

		for position, txSig := range dammV2Inits {
			fmt.Println("Refreshing Damm V2 init tx:\t", txSig)
			memo := fmt.Sprintf("Coin %s refresh Damm V2 init/migration %s", mint, txSig)
			sql, err := refreshAccount(ctx, rpcClient, damm_v2.NAME, position, memo, txSig)
			if err != nil {
				return fmt.Errorf("failed to refresh Damm V2 init/migration tx: %w", err)
			}
			updates = append(updates, sql)
		}

		for _, lock := range lockers {
			fmt.Println("Refreshing Escrow locker:\t", lock)
			memo := fmt.Sprintf("Coin %s refresh locker %s", mint, lock)
			sql, err := refreshAccount(ctx, rpcClient, locker.NAME, lock, memo, lock)
			if err != nil {
				fmt.Printf("failed to refresh locker: %v\n", err)
			}
			updates = append(updates, sql)
		}

		fmt.Print("\n\033[36m=== Generated SQL Updates ===\033[0m\n\n")

		sql := ""

		sql += fmt.Sprintf("-- Generated by sol_refresher coin command for mint %s\n\n", mint)
		sql += "BEGIN;\n\n"
		for _, update := range updates {
			sql += fmt.Sprintf("%s\n", update)
		}
		sql += "COMMIT;\n\n"
		sql += fmt.Sprintf("-- End of sol_refresher coin for mint %s\n", mint)
		fmt.Println(sql)

		databaseURL, err := cmd.Flags().GetString("database")
		if err != nil {
			return fmt.Errorf("failed to get database flag: %w", err)
		}

		confirm := false
		confirmPrompt := &survey.Confirm{
			Message: "Do you want to execute the generated SQL against the database?",
			Default: false,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}

		if confirm {
			config, err := pgxpool.ParseConfig(databaseURL)
			if err != nil {
				return fmt.Errorf("failed to parse database URL: %w", err)
			}
			pool, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				return fmt.Errorf("failed to create database pool: %w", err)
			}
			defer pool.Close()

			_, err = pool.Exec(ctx, sql)
			if err != nil {
				return fmt.Errorf("failed to execute SQL: %w", err)
			}

			fmt.Println("\n✅ SQL executed successfully")
		}

		return nil
	},
}

func findDbcPoolsForMint(ctx context.Context, rpcClient *rpc.Client, mint solana.PublicKey) ([]string, error) {
	fmt.Println("\nSearching for DBC pools...")
	result, err := rpcClient.GetProgramAccountsWithOpts(ctx, meteora_dbc.ProgramID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0,
					Bytes:  solana.Base58(meteora_dbc.POOL_DISCRIMINATOR[:]),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					// Base mint is after discriminator, VolatilityTracker, config and creator
					Offset: 8 + (8 + 8 + 16 + 16 + 16) + 32 + 32,
					Bytes:  solana.Base58(mint[:]),
				},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("No DBC pools found")
		return nil, nil
	}

	fmt.Printf("Found %d DBC pool(s) for mint %s\n", len(result), mint)
	results := make([]string, len(result))
	for i, acc := range result {
		results[i] = acc.Pubkey.String()
	}
	return results, nil
}

func findDammV2PoolsForMint(ctx context.Context, rpcClient *rpc.Client, mint solana.PublicKey) ([]string, error) {
	fmt.Println("\nSearching for Damm V2 pools...")
	result, err := rpcClient.GetProgramAccountsWithOpts(ctx, meteora_damm_v2.ProgramID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0,
					Bytes:  solana.Base58(meteora_damm_v2.POOL_DISCRIMINATOR[:]),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					// discriminator + sizeof(fee struct)
					// sizeof(fee struct) = sizeof(base fees) + 1 + 1 + 1 + 5 + sizeof(dynamic fees) + 16
					// sizeof(base fees) = 8 + 1 + 5 + 2 + 8 + 8 + 8 = 40
					// sizeof(dynamic fees) = 1 + 7 + 4 + 4 + 2 + 2 + 2 + 2 + 8 + 16 + 16 + 16 + 16 = 96
					// --> sizeof(fee struct) = 40 + 1 + 1 + 1 + 5 + 96 + 16 = 160
					Offset: 168, // 8 (discriminator) + 160 (fee struct) = 168
					Bytes:  solana.Base58(mint[:]),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					// discriminator + sizeof(fee struct) + 32 (Token A Mint)
					// sizeof(fee struct) = sizeof(base fees) + 1 + 1 + 1 + 5 + sizeof(dynamic fees) + 16
					// sizeof(base fees) = 8 + 1 + 5 + 2 + 8 + 8 + 8 = 40
					// sizeof(dynamic fees) = 1 + 7 + 4 + 4 + 2 + 2 + 2 + 2 + 8 + 16 + 16 + 16 + 16 = 96
					// --> sizeof(fee struct) = 40 + 1 + 1 + 1 + 5 + 96 + 16 = 160
					Offset: 200, // 8 (discriminator) + 160 (fee struct) + 32 = 200
					Bytes:  solana.Base58(solana.MustPublicKeyFromBase58(config.ProdMintAudio).Bytes()),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("No Damm V2 pools found")
		return nil, nil
	}

	fmt.Printf("Found %d Damm V2 pool(s) for mint %s\n", len(result), mint)
	results := make([]string, len(result))
	for i, acc := range result {
		results[i] = acc.Pubkey.String()
	}
	return results, nil

}

func findDammV2PositionsForPool(ctx context.Context, rpcClient *rpc.Client, pool solana.PublicKey) ([]string, error) {
	fmt.Println("\nSearching for Damm V2 positions...")
	result, err := rpcClient.GetProgramAccountsWithOpts(ctx, meteora_damm_v2.ProgramID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0,
					Bytes:  solana.Base58(meteora_damm_v2.POSITION_DISCRIMINATOR[:]),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					// Position pool is after discriminator
					Offset: 8,
					Bytes:  solana.Base58(pool[:]),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get program accounts: %w", err)
	}

	if len(result) == 0 {
		fmt.Println("No Damm V2 positions found")
		return nil, nil
	}

	fmt.Printf("Found %d Damm V2 position(s) for pool %s\n", len(result), pool)
	results := make([]string, 0)
	for _, acc := range result {
		results = append(results, acc.Pubkey.String())
	}
	return results, nil
}

func findLockersForMintAndDbcPools(ctx context.Context, rpcClient *rpc.Client, mint solana.PublicKey, dbcPools []string) ([]string, error) {
	fmt.Println("\nChecking Escrow lockers...")
	temp := make([]string, 0)
	for _, pool := range dbcPools {
		poolKey := solana.MustPublicKeyFromBase58(pool)
		base := meteora_dbc.DeriveBaseKeyForEscrow(poolKey)
		escrow := meteora_dbc.DeriveEscrow(base)
		acc, err := rpcClient.GetAccountInfo(ctx, escrow)
		if err != nil && !errors.Is(err, rpc.ErrNotFound) {
			return nil, fmt.Errorf("failed to get account info for locker %s: %w", escrow, err)
		}
		if acc != nil {
			temp = append(temp, escrow.String())
		}
	}
	locker := meteora_dbc.DeriveEscrow(mint)
	acc, err := rpcClient.GetAccountInfo(ctx, locker)
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return nil, fmt.Errorf("failed to get account info for locker %s: %w", locker, err)
	}
	if acc != nil {
		temp = append(temp, locker.String())
	}
	if len(temp) == 0 {
		fmt.Println("No lockers found")
	} else {
		fmt.Printf("Found %d locker(s)\n", len(temp))
	}
	return temp, nil
}

func findMigrationOrInitForPosition(ctx context.Context, rpcClient *rpc.Client, position solana.PublicKey) (*string, error) {
	fmt.Printf("\nSearching for DBC migrations or inits for position %s...\n", position)
	signatures, err := rpcClient.GetSignaturesForAddress(ctx, position)
	if err != nil {
		return nil, fmt.Errorf("failed to get signatures for address %s: %w", position, err)
	}
	if len(signatures) == 0 {
		fmt.Println("No DBC migrations or inits found")
		return nil, nil
	}

	fmt.Printf("Checking %d transactions for DBC migrations or inits...\n", len(signatures))

	version := uint64(0)
	for _, sig := range signatures {
		signature := sig.Signature.String()
		txRes, err := rpcClient.GetTransaction(ctx, sig.Signature, &rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &version,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction %s: %w", sig.Signature, err)
		}
		if txRes == nil || txRes.Transaction == nil {
			fmt.Println("Transaction result or transaction is nil for signature:", sig.Signature)
			continue
		}

		// Decode the transaction
		tx, err := txRes.Transaction.GetTransaction()
		if err != nil {
			return nil, fmt.Errorf("failed to decode transaction: %w", err)
		}

		// Check if the transaction contains a DBC migration or init instruction
		for instructionIndex, instruction := range tx.Message.Instructions {
			if int(instruction.ProgramIDIndex) >= len(tx.Message.AccountKeys) {
				continue
			}
			programID := tx.Message.AccountKeys[instruction.ProgramIDIndex]
			switch programID {
			case meteora_dbc.ProgramID:
				{
					accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
					if err != nil {
						return nil, fmt.Errorf("error resolving instruction accounts: %w", err)
					}

					inst, err := meteora_dbc.DecodeInstruction(accounts, instruction.Data)
					if err != nil {
						// Ignore unknown instruction types.
						// Not all DBC instruction types are implemented yet.
						// See: solana/spl/programs/meteora_dbc/instruction.go
						// See: https://github.com/gagliardetto/binary/blob/v0.8.0/variant.go#L315
						if strings.Contains(err.Error(), "no known type for type") {
							continue // ignore unknown instruction types
						}
						return nil, fmt.Errorf("error decoding meteora_dbc instruction %d: %w", instructionIndex, err)
					}

					switch inst.TypeID {
					case meteora_dbc.InstructionImplDef.TypeID(meteora_dbc.Instruction_MigrationDammV2):
						{
							return &signature, nil
						}
					}
				}
			case meteora_damm_v2.ProgramID:
				{
					accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
					if err != nil {
						return nil, fmt.Errorf("error resolving instruction accounts: %w", err)
					}

					inst, err := meteora_damm_v2.DecodeInstruction(accounts, instruction.Data)
					if err != nil {
						// Ignore unknown instruction types.
						// Not all DammV2 instruction types are implemented yet.
						// See: solana/spl/programs/meteora_damm_v2/instruction.go
						// See: https://github.com/gagliardetto/binary/blob/v0.8.0/variant.go#L315
						if strings.Contains(err.Error(), "no known type for type") {
							continue // ignore unknown instruction types
						}
						return nil, fmt.Errorf("error decoding meteora_damm_v2 instruction %d: %w", instructionIndex, err)
					}

					switch inst.TypeID {
					case meteora_damm_v2.InstructionImplDef.TypeID(meteora_damm_v2.Instruction_InitializeCustomPool):
						{
							return &signature, nil
						}
					}
				}
			}
		}
	}
	fmt.Println("No DBC migrations or inits found")
	return nil, nil
}

func refreshAccount(ctx context.Context, rpcClient *rpc.Client, indexer string, address string, memo string, filterName string) (string, error) {
	return refreshAccountWithTx(ctx, rpcClient, indexer, address, memo, filterName, solana.Signature{})
}

func refreshAccountWithTx(ctx context.Context, rpcClient *rpc.Client, indexer string, address string, memo string, filterName string, txSig solana.Signature) (string, error) {
	pubKey := solana.MustPublicKeyFromBase58(address)

	accRes, err := rpcClient.GetAccountInfo(ctx, pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to get account info for address %s: %w", address, err)
	}

	if txSig.IsZero() {
		limit := 1
		txRes, err := rpcClient.GetSignaturesForAddressWithOpts(ctx, pubKey, &rpc.GetSignaturesForAddressOpts{
			Limit: &limit,
		})
		if err != nil {
			return "", fmt.Errorf("failed to get signatures for address %s: %w", address, err)
		}
		if len(txRes) == 0 {
			return "", fmt.Errorf("no transactions found for address %s", address)
		}
		txSig = txRes[0].Signature
	}

	slot := accRes.Context.Slot

	update := &pb.SubscribeUpdate{
		Filters: []string{filterName},
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey:       pubKey.Bytes(),
					Lamports:     accRes.Value.Lamports,
					Owner:        accRes.Value.Owner.Bytes(),
					Data:         accRes.Value.Data.GetBinary(),
					Executable:   accRes.Value.Executable,
					RentEpoch:    accRes.Value.RentEpoch.Uint64(),
					TxnSignature: txSig[:],
				},
				Slot: slot,
			},
		},
	}
	json, err := protojson.Marshal(update)
	if err != nil {
		return "", fmt.Errorf("failed to marshal account to JSON: %w", err)
	}

	sql := fmt.Sprintf("INSERT INTO sol_retry_queue (indexer, update_message, error) VALUES ('%s', '%s', '%s');",
		indexer,
		string(json),
		memo,
	)

	return fmt.Sprintf("-- %s\n%s\n", memo, sql), nil
}

func selectAccounts(options []string) []string {
	if len(options) == 0 {
		return nil
	}

	selected := make([]string, 0)

	if len(options) == 1 {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Refresh account %s?", options[0]),
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			fmt.Printf("prompt failed: %v\n", err)
			return nil
		}
		if confirm {
			selected = append(selected, options[0])
		}
		return selected
	}

	prompt := &survey.MultiSelect{
		Message: "Select the account(s) to refresh:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		fmt.Printf("prompt failed: %v\n", err)
		return nil
	}

	return selected
}

func init() {
	coinCmd.Flags().String("dbc-pool", "", "The DBC pool address")
	coinCmd.Flags().String("dbc-config", "", "The DBC config address")
	coinCmd.Flags().BoolP("update-coin", "u", false, "Update the coin's Damm V2 pool reference in the database")
	coinCmd.Flags().String("damm-v2-pool", "", "The Damm V2 pool address")
	coinCmd.Flags().String("damm-v2-position", "", "The Damm V2 position address")
	coinCmd.Flags().String("locker", "", "The locker address")
	coinCmd.Flags().BoolP("all", "y", false, "Skip prompts and use all found accounts")
	coinCmd.Flags().BoolP("skip", "s", false, "Skip prompts and do not refresh unlisted accounts")
	coinCmd.Flags().String("database", "postgres://postgres:postgres@localhost:5432/discovery_provider_1?sslmode=disable", "Database connection string")
	rootCmd.AddCommand(coinCmd)
}
