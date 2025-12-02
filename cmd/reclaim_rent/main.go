package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"api.audius.co/solana/spl/programs/claimable_tokens"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
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

	offset := 0

	for {
		accounts, err := getTokenAccountsFromDatabase(ctx, pool, mint, 1000, offset)
		if err != nil {
			return fmt.Errorf("failed to get token accounts from database: %w", err)
		}

		if len(accounts) == 0 {
			fmt.Println("No more accounts to process.")
			break
		}
		fmt.Printf("Gathered %d accounts from db\n", len(accounts))

		offset += len(accounts)

		filtered, err := filterAccounts(ctx, rpcClient, accounts)
		if err != nil {
			return fmt.Errorf("failed to filter accounts: %w", err)
		}

		batchSize := 15
		i := 0

		for {
			batch := make([]DatabaseAccount, 0, batchSize)
			for j := i; j < i+batchSize && j < len(filtered); j++ {
				batch = append(batch, filtered[j])
			}
			if len(batch) == 0 {
				break
			}
			txSig, err := processBatch(ctx, rpcClient, batch, authority, destination, keypair)
			if err != nil {
				return fmt.Errorf("failed to process batch: %w", err)
			}
			if txSig != nil {
				fmt.Printf("Submitted transaction %s to reclaim rent for %d accounts\n", txSig.String(), len(batch))
			}
			time.Sleep(time.Second / 500 * 2) // Max 500 req/s (2 req per batch) to avoid rate limiting

			fmt.Printf("Processed %d/%d accounts\n", i+len(batch), len(filtered))
			i += batchSize
		}
	}
	return nil
}

func filterAccounts(ctx context.Context, rpcClient *rpc.Client, batch []DatabaseAccount) ([]DatabaseAccount, error) {
	accounts := make([]solana.PublicKey, 0, len(batch))
	for _, acct := range batch {
		accounts = append(accounts, solana.MustPublicKeyFromBase58(acct.Account))
	}

	res, err := rpcClient.GetMultipleAccountsWithOpts(ctx, accounts, &rpc.GetMultipleAccountsOpts{
		Encoding: solana.EncodingBase64,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	filtered := make([]DatabaseAccount, 0, len(batch))

	for i, acct := range res.Value {
		if acct == nil {
			fmt.Printf("Skipping account %s: account does not exist\n", batch[i].Account)
			continue
		}
		var tokenAccount token.Account
		err := bin.NewBorshDecoder(acct.Data.GetBinary()).Decode(&tokenAccount)
		if err != nil {
			fmt.Printf("Skipping account %s: failed to decode account data (%v)\n", batch[i].Account, err)
			continue
		}
		if tokenAccount.Amount != 0 {
			fmt.Printf("Skipping account %s: account balance is not zero\n", batch[i].Account)
			continue
		}
		filtered = append(filtered, batch[i])
	}
	return filtered, nil
}

func processBatch(ctx context.Context, rpcClient *rpc.Client, batch []DatabaseAccount, authority solana.PublicKey, destination solana.PublicKey, keypair solana.PrivateKey) (*solana.Signature, error) {
	instructions := make([]solana.Instruction, 0, len(batch))
	for _, acct := range batch {
		closeInstruction := claimable_tokens.NewCloseInstructionBuilder().
			SetUserBank(solana.MustPublicKeyFromBase58(acct.Account)).
			SetAuthority(authority).
			SetDestination(destination).
			SetEthAddress(common.HexToAddress(acct.EthereumAddress))
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

	txSig := tx.Signatures[0]
	_, err = rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error sending transaction %s: %v", txSig.String(), err)
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

type DatabaseAccount struct {
	Account         string
	EthereumAddress string
}

func getTokenAccountsFromDatabase(ctx context.Context, pool *pgxpool.Pool, mint solana.PublicKey, limit, offset int) ([]DatabaseAccount, error) {
	sql := `
		SELECT bank_account AS account, ethereum_address
		FROM user_bank_accounts
		LIMIT $1 OFFSET $2
	`
	rows, err := pool.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query token accounts: %w", err)
	}

	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByName[DatabaseAccount])
	if err != nil {
		return nil, fmt.Errorf("failed to collect token accounts: %w", err)
	}

	return accounts, nil
}
