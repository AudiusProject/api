package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"api.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
)

var reclaimRentCmd = &cobra.Command{
	Use:   "reclaim_rent <mint>",
	Short: "Reclaim rent from empty token accounts for a given mint",
	Args:  cobra.ExactArgs(1),
	RunE:  reclaimRent,
}

func main() {
	err := reclaimRentCmd.Execute()
	if err != nil {
		fmt.Println("Error executing command:", err)
	}
}

func init() {
	reclaimRentCmd.Flags().StringP("rpc", "r", "https://api.mainnet-beta.solana.com", "The Solana RPC endpoint to use")
	reclaimRentCmd.Flags().StringP("database", "c", "postgres://postgres:postgres@localhost:5432/discovery_provider_1?sslmode=disable", "Database connection string")
	reclaimRentCmd.Flags().StringP("keypair", "k", "~/.config/solana/id.json", "The wallet to use as fee payer for transactions")
	reclaimRentCmd.Flags().StringP("destination", "d", "", "The recipient of reclaimed rent (defaults to fee payer)")
	reclaimRentCmd.Flags().StringP("program", "p", claimable_tokens.ProgramID.String(), "The claimable tokens program ID")
}

func reclaimRent(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rpcEndpoint, err := cmd.Flags().GetString("rpc")
	if err != nil {
		return fmt.Errorf("failed to get rpc flag: %w", err)
	}
	rpcClient := rpc.New(rpcEndpoint)

	databaseURL, err := cmd.Flags().GetString("database")
	if err != nil {
		return fmt.Errorf("failed to get database flag: %w", err)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer pool.Close()

	feePayerFlag, err := cmd.Flags().GetString("keypair")
	if err != nil {
		return fmt.Errorf("failed to get keypair flag: %w", err)
	}
	keypair, err := solana.PrivateKeyFromSolanaKeygenFile(feePayerFlag)
	if err != nil {
		return fmt.Errorf("failed to load keypair: %w", err)
	}

	destinationFlag, err := cmd.Flags().GetString("destination")
	if err != nil {
		return fmt.Errorf("failed to get destination flag: %w", err)
	}
	var destination solana.PublicKey
	if destinationFlag == "" {
		destination = keypair.PublicKey()
	} else {
		destination = solana.MustPublicKeyFromBase58(destinationFlag)
	}

	programIDFlag, err := cmd.Flags().GetString("program")
	if err != nil {
		return fmt.Errorf("failed to get program flag: %w", err)
	}
	claimable_tokens.SetProgramID(solana.MustPublicKeyFromBase58(programIDFlag))

	mint := solana.MustPublicKeyFromBase58(args[0])

	fmt.Println("Reclaiming rent for mint:", args[0])

	authority, _, err := claimable_tokens.DeriveAuthority(mint)
	if err != nil {
		return fmt.Errorf("failed to derive authority: %w", err)
	}

	var pageKey *string

	for {
		res, err := getEmptyTokenAccounts(ctx, rpcClient, mint, authority, pageKey)
		if err != nil {
			return fmt.Errorf("failed to get token accounts: %w", err)
		}

		pageKey = res.PaginationKey
		safePageKey := "<nil>"
		if pageKey != nil {
			safePageKey = *pageKey
		}

		fmt.Printf("Found %d empty token accounts for mint %s owned by authority %s on page %s\n", len(res.Accounts), mint.String(), authority.String(), safePageKey)

		i := 0
		batchSize := 15

		for {
			batch := make([]solana.PublicKey, 0, batchSize)
			for j := i; j < i+batchSize && j < len(res.Accounts); j++ {
				batch = append(batch, res.Accounts[j].Pubkey)
			}
			if len(batch) == 0 {
				break
			}
			txSig, err := processBatch(ctx, pool, rpcClient, batch, authority, destination, keypair)
			if err != nil {
				return fmt.Errorf("failed to process batch: %w", err)
			}
			if txSig != nil {
				fmt.Printf("Submitted transaction %s to reclaim rent for %d accounts\n", txSig.String(), len(batch))
			}
			time.Sleep(time.Second / 500 * 2) // Max 500 req/s (2 req per batch) to avoid rate limiting

			i += batchSize

			// TODO: do more than one batch
			fmt.Println("Processed one batch, exiting for now.")
			return nil
		}

		if pageKey == nil {
			break
		}
	}
	return nil
}

func processBatch(ctx context.Context, pool *pgxpool.Pool, rpcClient *rpc.Client, batch []solana.PublicKey, authority solana.PublicKey, destination solana.PublicKey, keypair solana.PrivateKey) (*solana.Signature, error) {
	ethAddresses, err := getEthAddressesFromAccounts(ctx, pool, batch)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH addresses: %w", err)
	}

	instructions := make([]solana.Instruction, 0, len(batch))
	for _, acct := range batch {
		if _, ok := ethAddresses[acct]; !ok {
			fmt.Printf("Skipping account %s: no associated eth address found\n", acct.String())
			continue
		}
		closeInstruction := claimable_tokens.NewCloseInstructionBuilder().
			SetUserBank(acct).
			SetAuthority(authority).
			SetDestination(destination).
			SetEthAddress(ethAddresses[acct])
		instructions = append(instructions, closeInstruction.Build())

	}

	if len(instructions) == 0 {
		fmt.Println("No valid accounts to process in this batch.")
		return nil, nil
	}

	blockhashResult, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("error getting recent blockhash: %v", err)
	}
	recentBlockhash := blockhashResult.Value.Blockhash

	tx, err := solana.NewTransaction(
		instructions,
		recentBlockhash,
		solana.TransactionPayer(keypair.PublicKey()),
	)
	if err != nil {
		return nil, fmt.Errorf("error building transaction: %v", err)
	}

	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(keypair.PublicKey()) {
			return &keypair
		}
		return nil
	})

	txSig, err := rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("error sending transaction: %v", err)
	}
	return &txSig, nil
}

func getEmptyTokenAccounts(ctx context.Context, client *rpc.Client, mint solana.PublicKey, owner solana.PublicKey, pageKey *string) (rpc.GetProgramAccountsV2Result, error) {
	mintOffset := uint64(0)
	ownerOffset := uint64(32)
	balanceOffset := uint64(64)
	balance := make([]byte, 8)

	dataSliceOffset := uint64(0)
	dataSliceLength := uint64(0)
	limit := uint64(10000)

	return client.GetProgramAccountsV2WithOpts(ctx, solana.TokenProgramID, &rpc.GetProgramAccountsV2Opts{
		GetProgramAccountsOpts: rpc.GetProgramAccountsOpts{
			DataSlice: &rpc.DataSlice{
				Offset: &dataSliceOffset,
				Length: &dataSliceLength,
			},
			Filters: []rpc.RPCFilter{
				{
					Memcmp: &rpc.RPCFilterMemcmp{
						Offset: mintOffset,
						Bytes:  mint[:],
					},
				},
				{
					Memcmp: &rpc.RPCFilterMemcmp{
						Offset: ownerOffset,
						Bytes:  owner[:],
					},
				},
				{
					Memcmp: &rpc.RPCFilterMemcmp{
						Offset: balanceOffset,
						Bytes:  balance,
					},
				},
				{
					DataSize: 165, // Standard SPL Token account size
				},
			},
		},
		PaginationKey: pageKey,
		Limit:         &limit,
	})
}

func getEthAddressesFromAccounts(ctx context.Context, pool *pgxpool.Pool, accounts []solana.PublicKey) (map[solana.PublicKey]common.Address, error) {
	sql := `
		SELECT account, ethereum_address
		FROM sol_claimable_accounts
		WHERE account = ANY($1)
	`
	rows, err := pool.Query(ctx, sql, accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to query eth addresses: %w", err)
	}
	defer rows.Close()

	result := make(map[solana.PublicKey]common.Address)
	for rows.Next() {
		var account string
		var ethAddress string
		if err := rows.Scan(&account, &ethAddress); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result[solana.MustPublicKeyFromBase58(account)] = common.HexToAddress(ethAddress)
	}
	return result, nil
}
