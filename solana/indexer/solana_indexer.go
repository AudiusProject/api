package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/logging"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"bridgerton.audius.co/solana/spl/programs/secp256k1"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"go.uber.org/zap"
)

type SolanaIndexer struct {
	rpcClient *rpc.Client

	config config.Config
	pool   *pgxpool.Pool

	logger *zap.Logger
}

// LaserStream from Helius only keeps the last 3000 slots
var MAX_SLOT_GAP = uint64(3000)

var BATCH_DELAY_MS = uint64(50)
var TRANSACTION_DELAY_MS = uint(5)

var OLD_MEMO_PROGRAM_ID = solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

// Creates a Solana indexer.
func New(config config.Config) *SolanaIndexer {
	logger := logging.NewZapLogger(config).
		With(zap.String("service", "SolanaIndexer"))

	rpcClient := rpc.New(config.SolanaConfig.RpcProviders[0])

	return &SolanaIndexer{
		rpcClient: rpcClient,
		logger:    logger,
		config:    config,
	}
}

// Starts the indexer.
func (s *SolanaIndexer) Start(ctx context.Context) error {
	connConfig, err := pgxpool.ParseConfig(s.config.WriteDbUrl)
	if err != nil {
		return fmt.Errorf("error parsing database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, connConfig)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	s.pool = pool

	grpcClient := NewGrpcClient(GrpcConfig{
		Server:               s.config.SolanaConfig.GrpcProvider,
		ApiToken:             s.config.SolanaConfig.GrpcToken,
		MaxReconnectAttempts: 5,
	}, pool)

	latestChainSlot, err := s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	for err != nil {
		s.logger.Error("failed to get latest chain slot - retrying", zap.Error(err))
		time.Sleep(time.Second * 5)
		latestChainSlot, err = s.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
	}

	getLastIndexedSlotSql := `SELECT slot FROM solana_indexer_checkpoint LIMIT 1`
	var lastIndexedSlot uint64
	if err := s.pool.QueryRow(ctx, getLastIndexedSlotSql).Scan(&lastIndexedSlot); err != nil {
		if err != pgx.ErrNoRows {
			s.logger.Error("error getting last indexed slot", zap.Error(err))
		}
	}

	// LaserStream has a max slot gap of 3000 slots. Backfill the indexer until
	// the latest chain slot is within MAX_SLOT_GAP of the last indexed slot,
	// then start the gRPC subscription from the last indexed slot.
	for latestChainSlot-lastIndexedSlot > MAX_SLOT_GAP {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		s.logger.Info("slot gap too large, backfilling indexer", zap.Uint64("latest_chain_slot", latestChainSlot), zap.Uint64("last_indexed_slot", lastIndexedSlot))
		var wg sync.WaitGroup
		for _, address := range []solana.PublicKey{
			s.config.SolanaConfig.MintAudio,
			s.config.SolanaConfig.RewardManagerProgramID,
			s.config.SolanaConfig.ClaimableTokensProgramID,
			s.config.SolanaConfig.PaymentRouterProgramID,
		} {
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
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
	subscription.Accounts = make(map[string]*pb.SubscribeRequestFilterAccounts)
	accountFilter := pb.SubscribeRequestFilterAccounts{
		Owner: []string{solana.TokenProgramID.String()},
		Filters: []*pb.SubscribeRequestFilterAccountsFilter{
			{
				Filter: &pb.SubscribeRequestFilterAccountsFilter_TokenAccountState{
					TokenAccountState: true,
				},
			},
			{
				Filter: &pb.SubscribeRequestFilterAccountsFilter_Memcmp{
					Memcmp: &pb.SubscribeRequestFilterAccountsFilterMemcmp{
						Offset: 0,
						Data: &pb.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
							Bytes: config.Cfg.SolanaConfig.MintAudio.Bytes(),
						},
					},
				},
			},
		},
	}
	subscription.Accounts["audioAccounts"] = &accountFilter
	if err := grpcClient.Subscribe(ctx, subscription, s.onMessage, s.onError); err != nil {
		s.logger.Error("error subscribing to gRPC server", zap.Error(err))
		return nil
	}

	s.logger.Info("listening for new transactions...")
	<-ctx.Done()
	return ctx.Err()
}

// Handles a message from the gRPC subscription.
func (s *SolanaIndexer) onMessage(ctx context.Context, msg *pb.SubscribeUpdate) {
	accUpdate := msg.GetAccount()
	if accUpdate != nil {
		txSig := solana.SignatureFromBytes(accUpdate.Account.TxnSignature)
		err := s.ProcessSignature(ctx, accUpdate.Slot, txSig)
		if err != nil {
			s.logger.Error("failed to process signature", zap.Error(err))
		}
	}
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
		select {
		case <-ctx.Done():
			return
		default:
		}
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

		for _, sig := range res {
			select {
			case <-ctx.Done():
				return
			default:
			}
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
			err := s.pool.QueryRow(context.Background(), checkSql, pgx.NamedArgs{
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

			err = s.ProcessSignature(ctx, sig.Slot, sig.Signature)
			if err != nil {
				logger.Error("failed to process signature", zap.Error(err))
			}

			lastIndexedSig = sig.Signature

			// sleep for a bit to avoid hitting rate limits
			time.Sleep(time.Millisecond * time.Duration(TRANSACTION_DELAY_MS))
		}

		before = lastIndexedSig
		logger.Info("finished transaction batch",
			zap.Int("count", len(res)),
			zap.String("before", before.String()),
		)

		// sleep for a bit to avoid hitting rate limits
		time.Sleep(time.Millisecond * time.Duration(BATCH_DELAY_MS))
	}
	logger.Info("backfill completed")
}

func (s SolanaIndexer) ProcessSignature(ctx context.Context, slot uint64, txSig solana.Signature) error {
	sqlTx, err := s.pool.Begin(ctx)
	defer sqlTx.Rollback(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin sql transaction: %w", err)
	}

	txRes, err := s.rpcClient.GetTransaction(
		ctx,
		txSig,
		&rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &rpc.MaxSupportedTransactionVersion0,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	tx, err := txRes.Transaction.GetTransaction()
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	err = s.ProcessTransaction(ctx, sqlTx, slot, txRes.Meta, tx, txRes.BlockTime.Time(), s.logger)
	if err != nil {
		return fmt.Errorf("failed to process transaction: %w", err)
	}

	err = sqlTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit sql transaction: %w", err)
	}
	return nil
}

func (s *SolanaIndexer) ProcessTransaction(
	ctx context.Context,
	db database.DBTX,
	slot uint64,
	meta *rpc.TransactionMeta,
	tx *solana.Transaction,
	blockTime time.Time,
	logger *zap.Logger,
) error {
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

	signature := tx.Signatures[0].String()
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
							err := insertClaimableAccount(ctx, db, claimableAccountsRow{
								signature:        signature,
								instructionIndex: instructionIndex,
								slot:             slot,
								mint:             createInst.Mint().PublicKey.String(),
								ethereumAddress:  strings.ToLower(createInst.EthAddress.Hex()),
								bankAccount:      createInst.UserBank().PublicKey.String(),
							})
							if err != nil {
								return fmt.Errorf("failed to insert claimable tokens account at instruction %d: %w", instructionIndex, err)
							}
							logger.Debug("claimable_tokens createTokenAccount",
								zap.String("mint", createInst.Mint().PublicKey.String()),
								zap.String("userBank", createInst.UserBank().PublicKey.String()),
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
								return fmt.Errorf("failed to resolve instruction accounts at instruction %d: %w", instructionIndex-1, err)
							}
							secpInstRaw, err := secp256k1.DecodeInstruction(accounts, secpInstruction.Data)
							if err != nil {
								return fmt.Errorf("failed to decode secp256k1 instruction %d: %w", instructionIndex-1, err)
							}
							if secpInst, ok := secpInstRaw.Impl.(*secp256k1.Secp256k1Instruction); ok {
								dec := bin.NewBinDecoder(secpInst.SignatureDatas[0].Message)
								err := dec.Decode(&signedData)
								if err != nil {
									return fmt.Errorf("failed to parse signed transfer data at instruction %d: %w", instructionIndex-1, err)
								}
							}
							err = insertClaimableAccountTransfer(ctx, db, claimableAccountTransfersRow{
								signature:        signature,
								instructionIndex: instructionIndex,
								amount:           signedData.Amount,
								slot:             slot,
								fromAccount:      transferInst.SenderUserBank().PublicKey.String(),
								toAccount:        transferInst.Destination().PublicKey.String(),
								senderEthAddress: strings.ToLower(transferInst.SenderEthAddress.Hex()),
							})
							if err != nil {
								return fmt.Errorf("failed to insert claimable tokens transfer at instruction %d: %w", instructionIndex, err)
							}
							logger.Info("claimable_tokens transfer",
								zap.String("ethAddress", transferInst.SenderEthAddress.String()),
								zap.String("userBank", transferInst.SenderUserBank().PublicKey.String()),
								zap.String("destination", transferInst.Destination().PublicKey.String()),
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
						disbursementIdParts := strings.Split(claimInst.DisbursementId, ":")
						err := insertRewardDisbursement(ctx, db, rewardDisbursementsRow{
							signature:        signature,
							instructionIndex: instructionIndex,
							amount:           claimInst.Amount,
							slot:             slot,
							userBank:         claimInst.DestinationUserBankAccount().PublicKey.String(),
							challengeId:      disbursementIdParts[0],
							specifier:        strings.Join(disbursementIdParts[1:], ":"),
						})
						if err != nil {
							return fmt.Errorf("failed to insert reward disbursement at instruction %d: %w", instructionIndex, err)
						}
						logger.Info("reward_manager evaluateAttestations",
							zap.String("ethAddress", claimInst.RecipientEthAddress.String()),
							zap.String("userBank", claimInst.DestinationUserBankAccount().PublicKey.String()),
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
						for i, account := range routeInst.GetDestinations() {
							insertPayment(ctx, db, paymentRow{
								signature:        signature,
								instructionIndex: instructionIndex,
								amount:           routeInst.Amounts[i],
								slot:             slot,
								routeIndex:       i,
								toAccount:        account.PublicKey.String(),
							})
						}

						parsedPurchaseMemo, ok := findNextPurchaseMemo(tx, instructionIndex)
						if ok {
							parsedLocationMemo := findNextLocationMemo(tx, instructionIndex)
							isValid, err := validatePurchase(ctx, s.config, db, routeInst, parsedPurchaseMemo, blockTime)
							if err != nil {
								logger.Error("invalid purchase", zap.Error(err))
								// continue - insert the purchase as invalid for record keeping
							}

							insertPurchase(ctx, db, purchaseRow{
								signature:          signature,
								instructionIndex:   instructionIndex,
								amount:             routeInst.TotalAmount,
								slot:               slot,
								fromAccount:        routeInst.GetSender().PublicKey.String(),
								parsedPurchaseMemo: parsedPurchaseMemo,
								parsedLocationMemo: parsedLocationMemo,
								isValid:            isValid,
							})
						}

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
			balanceChange: *bal,
			signature:     tx.Signatures[0].String(),
		}, logger)
	}
	return nil
}

func findNextPurchaseMemo(tx *solana.Transaction, instructionIndex int) (parsedPurchaseMemo, bool) {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			memo := inst.Data
			parts := strings.Split(string(memo), ":")
			if len(parts) > 3 {
				contentType := parts[0]
				contentId, err := strconv.Atoi(parts[1])
				if err != nil {
					continue
				}
				validAfterBlocknumber, err := strconv.Atoi(parts[2])
				if err != nil {
					continue
				}
				buyerUserId, err := strconv.Atoi(parts[3])
				if err != nil {
					continue
				}
				accessType := "stream"
				if len(parts) > 4 {
					accessType = parts[4]
				}
				parsed := parsedPurchaseMemo{
					contentType:           contentType,
					contentId:             contentId,
					validAfterBlocknumber: validAfterBlocknumber,
					buyerUserId:           buyerUserId,
					accessType:            accessType,
				}
				return parsed, true
			}
		}
	}
	return parsedPurchaseMemo{}, false
}

func findNextLocationMemo(tx *solana.Transaction, instructionIndex int) parsedLocationMemo {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			memo := inst.Data
			if len(memo) > 3 && string(memo[0:3]) == "geo" {
				var parsed parsedLocationMemo
				err := json.Unmarshal(memo[4:], &parsed)
				if err != nil {
					continue
				}
				return parsed
			}
		}
	}
	return parsedLocationMemo{}
}

// Gets a map of account address to balance change from the given transaction.
func getTokenBalanceChanges(meta *rpc.TransactionMeta, tx *solana.Transaction) (map[string]*balanceChange, error) {
	balanceChanges := make(map[string]*balanceChange)

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

		balanceChanges[acc.String()] = &balanceChange{
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
			b = &balanceChange{
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
