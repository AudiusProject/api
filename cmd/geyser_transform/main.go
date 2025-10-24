package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"api.audius.co/config"
	"api.audius.co/solana/indexer/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"github.com/urfave/cli/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	cmd := &cli.Command{
		Name:  "geyser_transform",
		Usage: "Makes a Yellowstone gRPC SubscribeUpdate to insert into the retry queue",

		Commands: []*cli.Command{
			{
				Name: "tx",
				Usage: "Fetches a transaction by signature from the given RPC and transforms it " +
					"into a Yellowstone gRPC SubscribeUpdate message for insertion into the retry queue",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "indexer",
						UsageText: "The indexer name",
						Config: cli.StringConfig{
							TrimSpace: true,
						},
					},
					&cli.StringArg{
						Name:      "signature",
						UsageText: "The transaction signature to fetch",
						Config: cli.StringConfig{
							TrimSpace: true,
						},
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "rpc",
						Usage: "The RPC URL to use",
						Value: config.Cfg.SolanaConfig.RpcProviders[0],
					},
					&cli.StringFlag{
						Name:  "note",
						Usage: "An optional note to insert in the error column",
						Value: "manually inserted transaction",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					rpcClient := rpc.New(cmd.String("rpc"))
					txSig := solana.MustSignatureFromBase58(cmd.StringArg("signature"))

					// Fetch the transaction from the RPC
					maxSupportedTransactionVersion := uint64(0)
					txRes, err := rpcClient.GetTransaction(ctx, txSig, &rpc.GetTransactionOpts{
						Commitment:                     rpc.CommitmentConfirmed,
						MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
					})
					if err != nil {
						return err
					}

					// Decode the transaction
					tx, err := txRes.Transaction.GetTransaction()
					if err != nil {
						return fmt.Errorf("failed to decode transaction: %w", err)
					}

					// Add the lookup table accounts to the message accounts
					tx = common.ResolveLookupTables(ctx, rpcClient, tx, txRes.Meta)

					// Transform to protobuf
					pbTx := TransformTransaction(tx)
					pbMeta := TransformTransactionMeta(txRes.Meta)

					// Create update
					update := &pb.SubscribeUpdate{
						UpdateOneof: &pb.SubscribeUpdate_Transaction{
							Transaction: &pb.SubscribeUpdateTransaction{
								Transaction: &pb.SubscribeUpdateTransactionInfo{
									Signature:   txSig[:],
									IsVote:      tx.IsVote(),
									Transaction: pbTx,
									Meta:        pbMeta,
									Index:       0,
								},
								Slot: txRes.Slot,
							},
						},
					}

					json, err := protojson.Marshal(update)
					if err != nil {
						return fmt.Errorf("failed to marshal transaction to JSON: %w", err)
					}

					fmt.Printf("INSERT INTO sol_retry_queue (indexer, update_message, error) VALUES ('%s', '%s', '%s');\n",
						cmd.StringArg("indexer"),
						string(json),
						cmd.String("note"),
					)

					return nil
				},
			},
			{
				Name: "acc",
				Usage: "Fetches an account by pubkey from the given RPC and transforms it " +
					"into a Yellowstone gRPC SubscribeUpdate message for insertion into the retry queue",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "indexer",
						UsageText: "The indexer name",
						Config: cli.StringConfig{
							TrimSpace: true,
						},
					},
					&cli.StringArg{
						Name:      "pubkey",
						UsageText: "The account pubkey to fetch",
						Config: cli.StringConfig{
							TrimSpace: true,
						},
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "tx",
						Usage: "The transaction signature associated with the account fetch " +
							"(optional)",
						Value: "",
					},
					&cli.StringFlag{
						Name:  "rpc",
						Usage: "The RPC URL to use",
						Value: config.Cfg.SolanaConfig.RpcProviders[0],
					},
					&cli.StringFlag{
						Name:  "note",
						Usage: "An optional note to insert in the error column",
						Value: "manually inserted account",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					rpcClient := rpc.New(cmd.String("rpc"))
					pubkey := solana.MustPublicKeyFromBase58(cmd.StringArg("pubkey"))

					// Fetch the account from the RPC
					accRes, err := rpcClient.GetAccountInfo(ctx, pubkey)
					if err != nil {
						return err
					}

					slot := accRes.Context.Slot

					// Get optional transaction signature
					sigArg := cmd.String("tx")
					var txSig *solana.Signature = nil
					if sigArg != "" {
						s := solana.MustSignatureFromBase58(sigArg)
						txSig = &s

						maxSupportedTransactionVersion := uint64(0)
						txRes, err := rpcClient.GetTransaction(ctx, *txSig, &rpc.GetTransactionOpts{
							Commitment:                     rpc.CommitmentConfirmed,
							MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
						})
						if err != nil {
							return fmt.Errorf("failed to fetch transaction for account: %w", err)
						}

						slot = txRes.Slot
					} else {
						// prompt the user to confirm they don't want a transaction signature
						fmt.Println("Transaction signature not specified. This may cause issues in indexing...")
						fmt.Println("Are you sure you don't want to specify a transaction signature for the update? (y/n)")
						var response string
						fmt.Scanln(&response)
						if response != "yes" && response != "y" {
							return fmt.Errorf("Aborted by user")
						}
					}

					// Create update
					update := &pb.SubscribeUpdate{
						Filters: []string{pubkey.String()},
						UpdateOneof: &pb.SubscribeUpdate_Account{
							Account: &pb.SubscribeUpdateAccount{
								Account: &pb.SubscribeUpdateAccountInfo{
									Pubkey:       pubkey.Bytes(),
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
						return fmt.Errorf("failed to marshal account to JSON: %w", err)
					}

					fmt.Printf("INSERT INTO sol_retry_queue (indexer, update_message, error) VALUES ('%s', '%s', '%s');\n",
						cmd.StringArg("indexer"),
						string(json),
						cmd.String("note"),
					)

					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		panic(err)
	}
}

func TransformTransaction(tx *solana.Transaction) *pb.Transaction {
	pbTx := &pb.Transaction{
		Signatures: make([][]byte, len(tx.Signatures)),
	}
	for i, sig := range tx.Signatures {
		pbTx.Signatures[i] = sig[:]
	}
	msg := &pb.Message{
		Header: &pb.MessageHeader{
			NumRequiredSignatures:       uint32(tx.Message.Header.NumRequiredSignatures),
			NumReadonlySignedAccounts:   uint32(tx.Message.Header.NumReadonlySignedAccounts),
			NumReadonlyUnsignedAccounts: uint32(tx.Message.Header.NumReadonlyUnsignedAccounts),
		},
		AccountKeys:         make([][]byte, len(tx.Message.AccountKeys)),
		RecentBlockhash:     tx.Message.RecentBlockhash[:],
		Instructions:        make([]*pb.CompiledInstruction, len(tx.Message.Instructions)),
		Versioned:           tx.Message.IsVersioned(),
		AddressTableLookups: make([]*pb.MessageAddressTableLookup, len(tx.Message.AddressTableLookups)),
	}
	for i, key := range tx.Message.AccountKeys {
		msg.AccountKeys[i] = key.Bytes()
	}
	for i, instr := range tx.Message.Instructions {
		accounts := make([]uint8, len(instr.Accounts))
		for j, acctIdx := range instr.Accounts {
			accounts[j] = uint8(acctIdx)
		}
		msg.Instructions[i] = &pb.CompiledInstruction{
			ProgramIdIndex: uint32(instr.ProgramIDIndex),
			Data:           instr.Data,
			Accounts:       accounts,
		}
	}
	for i, lookup := range tx.Message.AddressTableLookups {
		msg.AddressTableLookups[i] = &pb.MessageAddressTableLookup{
			AccountKey:      lookup.AccountKey.Bytes(),
			WritableIndexes: lookup.WritableIndexes,
			ReadonlyIndexes: lookup.ReadonlyIndexes,
		}
	}
	pbTx.Message = msg
	return pbTx
}

func TransformTransactionMeta(meta *rpc.TransactionMeta) *pb.TransactionStatusMeta {
	var errBytes []byte
	if meta.Err != nil {
		errBytes, _ = json.Marshal(meta.Err)
	}
	pbMeta := &pb.TransactionStatusMeta{
		Err:                     &pb.TransactionError{Err: errBytes},
		Fee:                     meta.Fee,
		PreBalances:             meta.PreBalances,
		PostBalances:            meta.PostBalances,
		InnerInstructions:       make([]*pb.InnerInstructions, len(meta.InnerInstructions)),
		LogMessages:             meta.LogMessages,
		LogMessagesNone:         meta.LogMessages == nil,
		PreTokenBalances:        make([]*pb.TokenBalance, len(meta.PreTokenBalances)),
		PostTokenBalances:       make([]*pb.TokenBalance, len(meta.PostTokenBalances)),
		Rewards:                 make([]*pb.Reward, len(meta.Rewards)),
		LoadedWritableAddresses: make([][]byte, len(meta.LoadedAddresses.Writable)),
		LoadedReadonlyAddresses: make([][]byte, len(meta.LoadedAddresses.ReadOnly)),
		ReturnData: &pb.ReturnData{
			ProgramId: meta.ReturnData.ProgramId.Bytes(),
			Data:      meta.ReturnData.Data.Content,
		},
	}
	for i, inner := range meta.InnerInstructions {
		pbInner := &pb.InnerInstructions{
			Index:        uint32(inner.Index),
			Instructions: make([]*pb.InnerInstruction, len(inner.Instructions)),
		}
		for j, instr := range inner.Instructions {
			accounts := make([]uint8, len(instr.Accounts))
			for k, acctIdx := range instr.Accounts {
				accounts[k] = uint8(acctIdx)
			}
			pbInner.Instructions[j] = &pb.InnerInstruction{
				ProgramIdIndex: uint32(instr.ProgramIDIndex),
				Data:           instr.Data,
				Accounts:       accounts,
			}
		}
		pbMeta.InnerInstructions[i] = pbInner
	}
	for i, tb := range meta.PreTokenBalances {
		pbMeta.PreTokenBalances[i] = &pb.TokenBalance{
			AccountIndex: uint32(tb.AccountIndex),
			Mint:         tb.Mint.String(),
			UiTokenAmount: &pb.UiTokenAmount{
				Amount:         tb.UiTokenAmount.Amount,
				Decimals:       uint32(tb.UiTokenAmount.Decimals),
				UiAmount:       *tb.UiTokenAmount.UiAmount,
				UiAmountString: tb.UiTokenAmount.UiAmountString,
			},
		}
	}
	for i, tb := range meta.PostTokenBalances {
		pbMeta.PostTokenBalances[i] = &pb.TokenBalance{
			AccountIndex: uint32(tb.AccountIndex),
			Mint:         tb.Mint.String(),
			UiTokenAmount: &pb.UiTokenAmount{
				Amount:         tb.UiTokenAmount.Amount,
				Decimals:       uint32(tb.UiTokenAmount.Decimals),
				UiAmount:       *tb.UiTokenAmount.UiAmount,
				UiAmountString: tb.UiTokenAmount.UiAmountString,
			},
		}
	}
	for i, reward := range meta.Rewards {
		rewardType := pb.RewardType_Unspecified
		switch reward.RewardType {
		case "Fee":
			rewardType = pb.RewardType_Fee
		case "Rent":
			rewardType = pb.RewardType_Rent
		case "Voting":
			rewardType = pb.RewardType_Voting
		case "Staking":
			rewardType = pb.RewardType_Staking
		}

		pbMeta.Rewards[i] = &pb.Reward{
			Pubkey:      reward.Pubkey.String(),
			Lamports:    reward.Lamports,
			PostBalance: reward.PostBalance,
			RewardType:  rewardType,
		}
	}
	for i, addr := range meta.LoadedAddresses.Writable {
		pbMeta.LoadedWritableAddresses[i] = addr.Bytes()
	}
	for i, addr := range meta.LoadedAddresses.ReadOnly {
		pbMeta.LoadedReadonlyAddresses[i] = addr.Bytes()
	}
	return pbMeta
}
