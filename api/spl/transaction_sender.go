package spl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

const (
	MAX_TRANSACTION_SIZE = 1232
)

var (
	// see https://github.com/solana-foundation/solana-web3.js/blob/maintenance/v1.x/src/utils/makeWebsocketUrl.ts
	URL_RE = regexp.MustCompile(`(?i)^[^:]+:\/\/([^:[]+|\[[^\]]+\])(:\d+)?(.*)`)
)

type TransactionSender struct {
	feePayers []solana.Wallet
	rpcUrls   []string
	client    *rpc.Client
}

func NewTransactionSender(feePayers []solana.Wallet, rpcProviders []string) *TransactionSender {
	return &TransactionSender{
		feePayers: feePayers,
		rpcUrls:   rpcProviders,
		client:    rpc.New(rpcProviders[0]),
	}
}

type InstructionError struct {
	Index              int
	Type               string
	Code               int
	EncodedTransaction string
}

func (e *InstructionError) Error() string {
	return fmt.Sprintf("instruction error. index: %d, type: %s, code: %d", e.Index, e.Type, e.Code)
}

type AddComputeBudgetLimitParams struct {
	Multiplier float64
	Padding    uint32
}

func (ts *TransactionSender) AddComputeBudgetLimit(ctx context.Context, tx *solana.TransactionBuilder, params AddComputeBudgetLimitParams) error {
	if tx == nil {
		return errors.New("can't add compute budget limit to nil transaction")
	}

	builtTx, err := tx.Build()
	if err != nil {
		return err
	}

	// Dummy sign tx to satisfy tx format
	_, err = builtTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		return &solana.NewWallet().PrivateKey
	})

	if err != nil {
		return err
	}

	simOpts := rpc.SimulateTransactionOpts{
		ReplaceRecentBlockhash: true,
		Commitment:             rpc.CommitmentConfirmed,
	}
	simResult, err := ts.client.SimulateTransactionWithOpts(ctx, builtTx, &simOpts)
	if err != nil {
		return err
	}

	if simResult.Value.Err != nil {
		str, err := json.Marshal(simResult.Value.Err)
		if err != nil {
			return fmt.Errorf("failed to set compute budget limit. simulation failed: %s", simResult.Value.Err)
		}
		instErr := InstructionError{}
		err = json.Unmarshal(str, &instErr)
		if err != nil {
			return fmt.Errorf("failed to set compute budget limit. simulation failed: %s", str)
		}
		return fmt.Errorf("failed to set compute budget limit. simulation failed: %w", &instErr)
	}

	if simResult.Value.UnitsConsumed == nil || *simResult.Value.UnitsConsumed == uint64(0) {
		return errors.New("failed to set compute budget limit. simulation failed")
	}

	computeUnits := uint32(float64(*simResult.Value.UnitsConsumed)*params.Multiplier) + params.Padding
	computeBudgetLimitInstr := computebudget.NewSetComputeUnitLimitInstruction(computeUnits)
	tx.AddInstruction(computeBudgetLimitInstr.Build())
	return nil
}

type AddPriorityFeesParams struct {
	Percentile int
	Multiplier float64
	Minimum    float64
	Maximum    float64
}

func (ts *TransactionSender) AddPriorityFees(ctx context.Context, tx *solana.TransactionBuilder, params AddPriorityFeesParams) error {
	if tx == nil {
		return errors.New("can't add priority fees to nil transaction")
	}

	recentFees, err := ts.client.GetRecentPrioritizationFees(ctx, solana.PublicKeySlice{})
	if err != nil {
		return err
	}

	sort.Slice(recentFees, func(i, j int) bool {
		return recentFees[i].PrioritizationFee > recentFees[j].PrioritizationFee
	})

	percentileIndex := (len(recentFees) - 1) * params.Percentile / 100
	lamportsPerCu := math.Max(float64(recentFees[percentileIndex].PrioritizationFee)*params.Multiplier, params.Minimum)
	if params.Maximum > 0 {
		lamportsPerCu = math.Min(lamportsPerCu, params.Maximum)
	}
	computeBudgetPriorityFeesInstr := computebudget.NewSetComputeUnitPriceInstruction(uint64(lamportsPerCu))
	tx.AddInstruction(computeBudgetPriorityFeesInstr.Build())
	return nil
}

func (ts *TransactionSender) GetFeePayer() solana.Wallet {
	return ts.feePayers[rand.IntN(len(ts.feePayers))]
}

func (ts *TransactionSender) SendTransactionWithRetries(ctx context.Context, txBuilder *solana.TransactionBuilder, commitment rpc.CommitmentType, opts rpc.TransactionOpts) (*solana.Signature, error) {
	latestBlockhashRes, err := ts.client.GetLatestBlockhash(ctx, commitment)
	if err != nil {
		return nil, err
	}
	txBuilder.SetRecentBlockHash(latestBlockhashRes.Value.Blockhash)

	tx, err := txBuilder.Build()
	if err != nil {
		return nil, err
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		for _, feePayer := range ts.feePayers {
			if key.Equals(feePayer.PublicKey()) {
				return &feePayer.PrivateKey
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	signature := tx.Signatures[0]
	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}

	websocketUrl, err := makeWebsocketUrl(ts.rpcUrls[0])
	if err != nil {
		return nil, err
	}

	wsClient, err := ws.Connect(ctx, websocketUrl)
	if err != nil {
		return nil, err
	}
	defer wsClient.Close()

	subscription, err := wsClient.SignatureSubscribe(signature, commitment)
	if err != nil {
		return nil, err
	}
	defer subscription.Unsubscribe()

	subCtx, cancel := context.WithCancel(context.Background())
	resChan := make(chan *ws.SignatureResult)
	errChan := make(chan error, 1)
	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			default:
				for _, rpcUrl := range ts.rpcUrls {
					tempClient := rpc.New(rpcUrl)
					go func() {
						maxRetries := uint(0)
						_, err := tempClient.SendRawTransactionWithOpts(subCtx, serializedTx, rpc.TransactionOpts{
							MaxRetries:    &maxRetries,
							SkipPreflight: true,
						})
						if err != nil {
							errChan <- err
							return
						}
					}()
				}
				time.Sleep(time.Second * 5)
			}
		}
	}()

	go func() {
		got, err := subscription.Recv(subCtx)
		if err != nil {
			errChan <- err
			return
		}

		if got == nil {
			errChan <- errors.New("failed to get transaction signature status")
			return
		}

		if got.Value.Err != nil {
			str, err := json.Marshal(got.Value.Err)
			if err != nil {
				errChan <- errors.New("failed to confirm transaction")
			}

			instErr := InstructionError{}
			err = json.Unmarshal(str, &instErr)
			if err != nil {
				errChan <- fmt.Errorf("failed to confirm transaction: %s", str)
			}

			errChan <- fmt.Errorf("failed to confirm transaction: %w", &instErr)
		}
		resChan <- got
	}()

	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			default:
				res, err := ts.client.GetBlockHeight(subCtx, rpc.CommitmentConfirmed)
				if err != nil {
					errChan <- err
					return
				}
				if latestBlockhashRes.Value.LastValidBlockHeight < res {
					errChan <- errors.New("failed to confirm transaction: TransactionExpiredBlockHeightExceeded")
					return
				}
				time.Sleep(time.Second * 5)
			}
		}
	}()

	select {
	case err := <-errChan:
		cancel()
		return nil, err
	case <-resChan:
		cancel()
		return &signature, nil
	}
}

func (e *InstructionError) UnmarshalJSON(b []byte) error {
	var rawData map[string]any
	if err := json.Unmarshal(b, &rawData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	parsedErr, ok := rawData["InstructionError"].([]any)
	if !ok || len(parsedErr) != 2 {
		return errors.New("invalid or missing InstructionError field")
	}

	instrIndex, ok := parsedErr[0].(float64)
	if !ok {
		return errors.New("invalid instruction index")
	}

	errStr, ok := parsedErr[1].(string)
	if ok {
		e.Index = int(instrIndex)
		e.Type = errStr
		return nil
	}

	errObj, ok := parsedErr[1].(map[string]any)
	if !ok {
		return errors.New("invalid error object")
	}

	customErr, ok := errObj["Custom"].(float64)
	if !ok {
		return errors.New("invalid custom error")
	}

	e.Index = int(instrIndex)
	e.Code = int(customErr)
	e.Type = "Custom"

	return nil
}

func makeWebsocketUrl(endpoint string) (string, error) {
	match := URL_RE.FindStringSubmatch(endpoint)

	if len(match) < 4 {
		return "", errors.New("bad rpc url")
	}
	hostish := match[1]
	portWithColon := match[2]
	rest := match[3]

	protocol := "ws:"
	if strings.HasPrefix(endpoint, "https") {
		protocol = "wss:"
	}

	startPort := -1
	if portWithColon != "" {
		parsedPort, err := strconv.ParseInt(portWithColon[1:], 10, 32)
		if err != nil {
			return "", err
		}
		startPort = int(parsedPort)
	}

	websocketPort := ""
	if startPort > 0 {
		websocketPort = strconv.FormatInt(int64(startPort+1), 10)
	}
	return fmt.Sprintf("%s//%s%s%s", protocol, hostish, websocketPort, rest), nil
}
