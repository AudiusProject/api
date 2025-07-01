package indexers

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/logging"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"bridgerton.audius.co/solana/spl/programs/secp256k1"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mr-tron/base58"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type SolanaIndexer struct {
	grpcClient *GrpcClient
	rpcClient  *rpc.Client
	logger     *zap.Logger

	addressesToWatch solana.PublicKeySlice
	tokenMints       solana.PublicKeySlice
}

// LaserStream from Helius only keeps the last 3000 slots
var MAX_SLOT_GAP = uint64(3000)

var BATCH_DELAY_MS = uint64(50)
var TRANSACTION_DELAY_MS = uint(5)

// Creates a Solana indexer.
func NewSolanaIndexer(config config.Config) *SolanaIndexer {
	logger := logging.NewZapLogger(config).
		With(zap.String("service", "SolanaIndexer"))

	grpcClient, err := NewGrpcClient(GrpcConfig{
		Server:               config.SolanaConfig.GrpcProvider,
		ApiToken:             config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
		WriteDbUrl:           config.WriteDbUrl,
	})
	if err != nil {
		logger.Fatal("failed to create gRPC client", zap.Error(err))
	}

	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])

	return &SolanaIndexer{
		grpcClient: grpcClient,
		rpcClient:  rpcClient,
		logger:     logger,
		tokenMints: solana.PublicKeySlice{
			config.SolanaConfig.MintAudio,
		},
		addressesToWatch: solana.PublicKeySlice{
			// config.SolanaConfig.MintAudio,
			// config.SolanaConfig.RewardManagerProgramID,
			// config.SolanaConfig.ClaimableTokensProgramID,
			config.SolanaConfig.PaymentRouterProgramID,
		},
	}
}

// Starts the indexer.
func (s *SolanaIndexer) Start(ctx context.Context) {
	latestChainSlot, err := s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	for err != nil {
		s.logger.Error("failed to get latest chain slot - retrying", zap.Error(err))
		time.Sleep(time.Second * 5)
		latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	}

	getLastIndexedSlotSql := `SELECT slot FROM solana_indexer_checkpoint LIMIT 1`
	var lastIndexedSlot uint64
	if err := s.grpcClient.pool.QueryRow(ctx, getLastIndexedSlotSql).Scan(&lastIndexedSlot); err != nil {
		if err != pgx.ErrNoRows {
			s.logger.Error("error getting last indexed slot", zap.Error(err))
		}
	}

	// LaserStream has a max slot gap of 3000 slots. Backfill the indexer until
	// the latest chain slot is within MAX_SLOT_GAP of the last indexed slot,
	// then start the gRPC subscription from the last indexed slot.
	for latestChainSlot-lastIndexedSlot > MAX_SLOT_GAP {
		s.logger.Info("slot gap too large, backfilling indexer", zap.Uint64("latest_chain_slot", latestChainSlot), zap.Uint64("last_indexed_slot", lastIndexedSlot))
		var wg sync.WaitGroup
		for _, address := range s.addressesToWatch {
			wg.Add(1)
			go func(addr solana.PublicKey) {
				defer wg.Done()
				s.backfillAddressTransactions(ctx, addr, lastIndexedSlot)
			}(address)
		}
		wg.Wait()
		lastIndexedSlot = latestChainSlot
		latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
		for err != nil {
			s.logger.Error("failed to get latest chain slot - retrying", zap.Error(err))
			time.Sleep(time.Second * 5)
			latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
		}
	}

	commitment := pb.CommitmentLevel_CONFIRMED
	subscription := &pb.SubscribeRequest{
		Commitment: &commitment,
		FromSlot:   &lastIndexedSlot,
	}
	subscription.Transactions = make(map[string]*pb.SubscribeRequestFilterTransactions, 0)
	vote := false
	failed := false
	accounts := make([]string, len(s.addressesToWatch))
	for i, address := range s.addressesToWatch {
		accounts[i] = address.String()
	}
	tokenFilter := pb.SubscribeRequestFilterTransactions{
		Vote:           &vote,
		AccountInclude: accounts,
		Failed:         &failed,
	}
	subscription.Transactions["tokenIndexer"] = &tokenFilter
	if err := s.grpcClient.Subscribe(subscription, s.onMessage, s.onError); err != nil {
		s.logger.Error("error subscribing to gRPC server", zap.Error(err))
		return
	}

	s.logger.Info("listening for new transactions...")
}

// Handles a message from the gRPC subscription.
func (s *SolanaIndexer) onMessage(msg *pb.SubscribeUpdate) {
	txUpdate := msg.GetTransaction()
	if txUpdate != nil && txUpdate.Transaction.Meta.Err == nil {
		txInfo := txUpdate.Transaction

		logger := s.logger.With(
			zap.String("signature", base58.Encode(txInfo.Signature)),
			zap.String("indexerSource", "grpc"),
		)
		logger.Debug("received transaction")
		tx := toTransaction(txInfo.Transaction)
		meta, err := toMeta(txInfo.Meta)
		if err != nil {
			logger.Error("failed to parse tx meta", zap.Error(err))
			return
		}
		ctx := context.Background()
		sqlTx, err := s.grpcClient.pool.Begin(ctx)
		defer sqlTx.Rollback(ctx)
		if err != nil {
			logger.Error("failed to being sql transaction", zap.Error(err))
			return
		}
		err = processTransaction(ctx, sqlTx, txUpdate.Slot, meta, tx, logger)
		if err != nil {
			logger.Error("failed to process tx", zap.Error(err))
			return
		}
		err = sqlTx.Commit(ctx)
		if err != nil {
			logger.Error("failed to commit sql transaction", zap.Error(err))
			return
		}
	}
}

type balanceChangeRow struct {
	balanceChange *BalanceChange
	account       string
	signature     string
	slot          uint64
}
type dbExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertBalanceChange(ctx context.Context, db dbExecutor, row balanceChangeRow, logger *zap.Logger) error {
	sql := `INSERT INTO solana_token_txs (account_address, mint, change, balance, signature, slot)
						VALUES (@account_address, @mint, @change, @balance, @signature, @slot)
						ON CONFLICT DO NOTHING`
	_, err := db.Exec(ctx, sql, pgx.NamedArgs{
		"account_address": row.account,
		"mint":            row.balanceChange.Mint,
		"change":          row.balanceChange.Change,
		"balance":         row.balanceChange.PostTokenBalance,
		"signature":       row.signature,
		"slot":            row.slot,
	})
	if logger != nil {
		logger.Debug("inserting balance change...",
			zap.String("account", row.account),
			zap.String("mint", row.balanceChange.Mint),
			zap.Uint64("balance", row.balanceChange.PostTokenBalance),
			zap.Int64("change", row.balanceChange.Change),
			zap.Uint64("slot", row.slot),
		)
	}
	return err
}

func (s *SolanaIndexer) onError(err error) {
	s.logger.Error("Error in solana indexer", zap.Error(err))
}

// Fetches and processes transactions for a given address.
// It should not update the last indexed slot. The caller will do that once
// backfill completes as the last indexed slot is for all backfillers and
// backfilling works in reverese chronological order.
func (s *SolanaIndexer) backfillAddressTransactions(ctx context.Context, address solana.PublicKey, lastIndexedSlot uint64) {
	var before solana.Signature
	var lastIndexedSig solana.Signature
	foundIntersection := false
	logger := s.logger.With(
		zap.String("indexerSource", "rpcBackfill"),
		zap.String("address", address.String()),
	)
	logger.Info("starting backfill for address")
	limit := 1000
	for !foundIntersection {
		res, err := s.rpcClient.GetSignaturesForAddressWithOpts(
			ctx,
			address,
			&rpc.GetSignaturesForAddressOpts{
				Commitment: rpc.CommitmentConfirmed,
				Before:     before,
				Limit:      &limit,
			},
		)
		if err != nil {
			logger.Error("failed to get signatures for address", zap.Error(err))
			continue
		}
		if len(res) == 0 {
			return
		}

		sqlTx, err := s.grpcClient.pool.Begin(ctx)
		defer sqlTx.Rollback(ctx)
		if err != nil {
			logger.Error("failed to begin transaction", zap.Error(err))
			continue
		}
		for _, sig := range res {
			if sig.Slot < lastIndexedSlot {
				foundIntersection = true
				break
			}
			logger := logger.With(zap.String("signature", sig.Signature.String()))

			// Skip error transactions
			if sig.Err != nil {
				lastIndexedSig = sig.Signature
				continue
			}

			// Skip zero signatures
			if sig.Signature.IsZero() {
				continue
			}

			// Skip existing transactions
			var exists bool
			checkSql := `SELECT EXISTS(SELECT 1 FROM solana_token_txs WHERE signature = @signature)`
			err := s.grpcClient.pool.QueryRow(context.Background(), checkSql, pgx.NamedArgs{
				"signature": sig.Signature,
			}).Scan(&exists)

			if err != nil {
				logger.Error("failed to check if signature exists", zap.Error(err))
				break
			}
			if exists {
				lastIndexedSig = sig.Signature
				continue
			}

			txRes, err := s.rpcClient.GetTransaction(
				ctx,
				sig.Signature,
				&rpc.GetTransactionOpts{
					Commitment:                     rpc.CommitmentConfirmed,
					MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
				},
			)
			if err != nil {
				logger.Error("failed to get transaction", zap.Error(err))
				break
			}

			meta := txRes.Meta
			parsedTx, err := txRes.Transaction.GetTransaction()
			if err != nil {
				logger.Error("failed to decode transaction", zap.Error(err))
				continue
			}

			err = processTransaction(ctx, sqlTx, txRes.Slot, meta, parsedTx, logger)
			if err != nil {
				logger.Error("failed to process transaction", zap.Error(err))
			}

			lastIndexedSig = sig.Signature

			// sleep for a bit to avoid hitting rate limits
			time.Sleep(time.Millisecond * time.Duration(TRANSACTION_DELAY_MS))
		}

		err = sqlTx.Commit(ctx)
		if err != nil {
			logger.Error("failed to commit transaction batch",
				zap.Error(err),
				zap.Int("count", len(res)),
				zap.String("before", before.String()),
			)
		} else {
			before = lastIndexedSig
			logger.Info("committed transaction batch",
				zap.Int("count", len(res)),
				zap.String("before", before.String()),
			)
		}
		// sleep for a bit to avoid hitting rate limits
		time.Sleep(time.Millisecond * time.Duration(BATCH_DELAY_MS))
	}
	logger.Info("backfill completed")
}

func processTransaction(ctx context.Context, db dbExecutor, slot uint64, meta *rpc.TransactionMeta, tx *solana.Transaction, logger *zap.Logger) error {
	if tx == nil {
		return fmt.Errorf("no transaction to process")
	}
	if meta == nil {
		return fmt.Errorf("missing tx meta")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Resolve address lookup tables
	addressTables := make(map[solana.PublicKey]solana.PublicKeySlice)
	writablePos := 0
	readonlyPos := 0
	for _, lu := range tx.Message.AddressTableLookups {
		addresses := make(solana.PublicKeySlice, 256)
		for _, idx := range lu.WritableIndexes {
			addresses[idx] = meta.LoadedAddresses.Writable[writablePos]
			writablePos += 1
		}
		for _, idx := range lu.ReadonlyIndexes {
			addresses[idx] = meta.LoadedAddresses.ReadOnly[readonlyPos]
			readonlyPos += 1
		}
		addressTables[lu.AccountKey] = addresses
	}
	tx.Message.SetAddressTables(addressTables)

	for instructionIndex, instruction := range tx.Message.Instructions {
		programId := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
		if err != nil {
			return fmt.Errorf("error resolving instruction accounts %d: %w", instructionIndex, err)
		}
		logger := logger.With(
			zap.String("programId", programId.String()),
			zap.Int("instructionIndex", instructionIndex),
		)
		switch programId {
		case claimable_tokens.ProgramID:
			{
				inst, err := claimable_tokens.DecodeInstruction(accounts, []byte(instruction.Data))
				if err != nil {
					return fmt.Errorf("error decoding claimable_tokens instruction %d: %w", instructionIndex, err)
				}
				switch inst.TypeID.Uint8() {
				case claimable_tokens.Instruction_CreateTokenAccount:
					{
						if createInst, ok := inst.Impl.(*claimable_tokens.CreateTokenAccount); ok {
							userBank, err := createInst.GetUserBank()
							if err != nil {
								return fmt.Errorf("failed to get user bank for claimable tokens instruction %d: %w", instructionIndex, err)
							}
							logger.Info("claimable_tokens createTokenAccount",
								zap.String("mint", createInst.Mint.String()),
								zap.String("userBank", userBank.String()),
								zap.String("ethAddress", createInst.EthAddress.String()),
							)
						}
					}
				case claimable_tokens.Instruction_Transfer:
					{
						if transferInst, ok := inst.Impl.(*claimable_tokens.Transfer); ok {
							var signedData claimable_tokens.SignedTransferData
							// The signed Secp256k1Instruction must be directly before the transfer
							secpInstruction := tx.Message.Instructions[instructionIndex-1]
							accounts, err := secpInstruction.ResolveInstructionAccounts(&tx.Message)
							if err != nil {
								return fmt.Errorf("error resolving instruction accounts %d: %w", instructionIndex-1, err)
							}
							secpInstRaw, err := secp256k1.DecodeInstruction(accounts, secpInstruction.Data)
							if err != nil {
								return fmt.Errorf("error decoding secp256k1 instruction %d: %w", instructionIndex-1, err)
							}
							if secpInst, ok := secpInstRaw.Impl.(*secp256k1.Secp256k1Instruction); ok {
								dec := bin.NewBinDecoder(secpInst.SignatureDatas[0].Message)
								err := dec.Decode(&signedData)
								if err != nil {
									return fmt.Errorf("error parsing signed transfer data %d: %w", instructionIndex-1, err)
								}
							}

							logger.Info("claimable_tokens transfer",
								zap.String("ethAddress", transferInst.SenderEthAddress.String()),
								zap.String("userBank", transferInst.GetSenderUserBank().PublicKey.String()),
								zap.String("destination", transferInst.GetDestination().PublicKey.String()),
								zap.Uint64("amount", signedData.Amount),
							)
						}
					}
				}
			}
		case reward_manager.ProgramID:
			{
				inst, err := reward_manager.DecodeInstruction(accounts, []byte(instruction.Data))
				if err != nil {
					return fmt.Errorf("error decoding reward_manager instruction %d: %w", instructionIndex, err)
				}
				switch inst.TypeID.Uint8() {
				case reward_manager.Instruction_EvaluateAttestations:
					if claimInst, ok := inst.Impl.(*reward_manager.EvaluateAttestation); ok {
						logger.Info("reward_manager evaluateAttestations",
							zap.String("ethAddress", claimInst.RecipientEthAddress.String()),
							zap.String("userBank", claimInst.GetDestinationUserBankAccount().PublicKey.String()),
							zap.Uint64("amount", claimInst.Amount),
							zap.String("disbursementId", claimInst.DisbursementId),
						)
					}
				}
			}
		case payment_router.ProgramID:
			{
				inst, err := payment_router.DecodeInstruction(accounts, []byte(instruction.Data))
				if err != nil {
					return fmt.Errorf("error decoding payment_router instruction %d: %w", instructionIndex, err)
				}
				switch inst.TypeID {
				case payment_router.InstructionImplDef.TypeID(payment_router.Instruction_Route):
					if routeInst, ok := inst.Impl.(*payment_router.Route); ok {
						logger.Info("payment_router route",
							zap.String("sender", routeInst.GetSender().PublicKey.String()),
							zap.Uint64s("amounts", routeInst.Amounts),
							zap.Strings("destinations", routeInst.GetDestinations().GetKeys().ToBase58()),
						)
					}
				}
			}
		}
	}
	balanceChanges, err := getTokenBalanceChanges(meta, tx)
	if err != nil {
		return err
	}
	for acc, bal := range balanceChanges {
		insertBalanceChange(ctx, db, balanceChangeRow{
			slot:          slot,
			account:       acc,
			balanceChange: bal,
			signature:     tx.Signatures[0].String(),
		}, logger)
	}
	return nil
}

type BalanceChange struct {
	Mint             string
	PreTokenBalance  uint64
	PostTokenBalance uint64
	Change           int64
}

// Gets a map of account address to balance change from the given transaction.
func getTokenBalanceChanges(meta *rpc.TransactionMeta, tx *solana.Transaction) (map[string]*BalanceChange, error) {
	balanceChanges := make(map[string]*BalanceChange)

	// Make a list of all accounts involved in the transaction
	allAccounts, err := tx.Message.AccountMetaList()
	if err != nil {
		return nil, err
	}
	// Pre balances
	for _, balance := range meta.PreTokenBalances {
		acc := allAccounts[balance.AccountIndex].PublicKey
		preBalance, err := strconv.ParseUint(balance.UiTokenAmount.Amount, 10, 64)
		if err != nil {
			return balanceChanges, err
		}

		balanceChanges[acc.String()] = &BalanceChange{
			Mint:            balance.Mint.String(),
			PreTokenBalance: preBalance,
		}
	}

	// Post balances and changes
	for _, balance := range meta.PostTokenBalances {
		acc := allAccounts[balance.AccountIndex].PublicKey
		postBalance, err := strconv.ParseUint(balance.UiTokenAmount.Amount, 10, 64)
		if err != nil {
			return balanceChanges, err
		}

		b := balanceChanges[acc.String()]
		if b == nil {
			b = &BalanceChange{
				Mint:            balance.Mint.String(),
				PreTokenBalance: 0,
			}
			balanceChanges[acc.String()] = b
		}
		b.PostTokenBalance = postBalance
		b.Change = int64(b.PostTokenBalance) - int64(b.PreTokenBalance)
	}
	return balanceChanges, nil
}
