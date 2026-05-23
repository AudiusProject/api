package indexer

// Batched contract reads via Multicall3
// (https://github.com/mds1/multicall, deployed at the same address on
// every EVM chain).
//
// Replaces N separate `eth_call` round-trips with one. The default
// stale-refresh tick reads balanceOf + totalStakedFor +
// getTotalDelegatorStake for each of 50 holders — 150 `eth_call`s
// otherwise, 1 with multicall.

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Multicall3 is deployed at the same address on every EVM chain via
// nick-method CREATE2. Universal across mainnet, testnets, L2s.
var multicall3Address = common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")

// multicallChunkSize bounds how many holders we send in a single
// multicall. 200 holders × 3 sub-calls = 600 sub-calls per outer
// `eth_call`. Well under typical RPC payload limits; keeps individual
// requests small enough to time out cleanly on network blips.
const multicallChunkSize = 200

// Call3 mirrors Multicall3's input struct.
type call3 struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

// Result3 mirrors Multicall3's output struct.
type result3 struct {
	Success    bool
	ReturnData []byte
}

var (
	aggregate3Selector = keccakSelector("aggregate3((address,bool,bytes)[])")
	multicall3Args     abi.Arguments
	multicall3Result   abi.Arguments
)

func init() {
	call3Type, err := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{
		{Name: "target", Type: "address"},
		{Name: "allowFailure", Type: "bool"},
		{Name: "callData", Type: "bytes"},
	})
	if err != nil {
		panic(fmt.Errorf("multicall: defining Call3[] type: %w", err))
	}
	result3Type, err := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{
		{Name: "success", Type: "bool"},
		{Name: "returnData", Type: "bytes"},
	})
	if err != nil {
		panic(fmt.Errorf("multicall: defining Result3[] type: %w", err))
	}
	multicall3Args = abi.Arguments{{Type: call3Type}}
	multicall3Result = abi.Arguments{{Type: result3Type}}
}

// totalAudioBalances batches balanceOf + totalStakedFor +
// getTotalDelegatorStake for each holder into a single Multicall3
// `aggregate3` call (chunked at multicallChunkSize holders per multicall).
// Returns the sum of the three values per holder, matching the Python
// discovery-provider's `associated_wallets_balance` semantics.
//
// Holders whose 3 sub-calls didn't all succeed are omitted from the
// returned map — same conservative posture as the previous per-wallet
// errgroup path, which short-circuited on any sub-call error rather than
// persist a partial sum.
func (e *EthIndexer) totalAudioBalances(ctx context.Context, holders []common.Address) (map[common.Address]*big.Int, error) {
	if len(holders) == 0 {
		return map[common.Address]*big.Int{}, nil
	}
	out := make(map[common.Address]*big.Int, len(holders))
	for i := 0; i < len(holders); i += multicallChunkSize {
		end := i + multicallChunkSize
		if end > len(holders) {
			end = len(holders)
		}
		chunk := holders[i:end]
		if err := e.multicallChunk(ctx, chunk, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (e *EthIndexer) multicallChunk(ctx context.Context, holders []common.Address, sink map[common.Address]*big.Int) error {
	calls := make([]call3, 0, len(holders)*3)
	for _, h := range holders {
		padded := common.LeftPadBytes(h.Bytes(), 32)
		calls = append(calls,
			call3{
				Target:       e.audioContract,
				AllowFailure: true,
				CallData:     append(append([]byte{}, balanceOfSelector...), padded...),
			},
			call3{
				Target:       e.stakingContract,
				AllowFailure: true,
				CallData:     append(append([]byte{}, totalStakedForSelector...), padded...),
			},
			call3{
				Target:       e.delegateManager,
				AllowFailure: true,
				CallData:     append(append([]byte{}, getTotalDelegatorStakeSelector...), padded...),
			},
		)
	}

	encoded, err := multicall3Args.Pack(calls)
	if err != nil {
		return fmt.Errorf("packing aggregate3 calls: %w", err)
	}
	data := append(append([]byte{}, aggregate3Selector...), encoded...)

	rawOut, err := e.httpClient.CallContract(ctx, ethereum.CallMsg{
		To:   &multicall3Address,
		Data: data,
	}, nil)
	if err != nil {
		return fmt.Errorf("multicall eth_call: %w", err)
	}

	decoded, err := multicall3Result.Unpack(rawOut)
	if err != nil {
		return fmt.Errorf("unpacking aggregate3 result: %w", err)
	}
	if len(decoded) != 1 {
		return fmt.Errorf("expected 1 top-level value in result, got %d", len(decoded))
	}

	// go-ethereum's abi package returns the tuple[] as a slice of
	// anonymous structs; coerce into our named result3 via reflection.
	results := *abi.ConvertType(decoded[0], new([]result3)).(*[]result3)

	if len(results) != len(calls) {
		return fmt.Errorf("multicall result count mismatch: %d vs %d", len(results), len(calls))
	}

	for i, h := range holders {
		b, ok := decodeUint(results[i*3+0])
		if !ok {
			continue
		}
		s, ok := decodeUint(results[i*3+1])
		if !ok {
			continue
		}
		d, ok := decodeUint(results[i*3+2])
		if !ok {
			continue
		}
		sum := new(big.Int).Add(b, s)
		sum.Add(sum, d)
		sink[h] = sum
	}
	return nil
}

// decodeUint extracts a uint256 from a single Multicall3 Result. Returns
// (0, false) on failure or empty data so the caller can skip the holder
// (we'd rather not persist a partial sum).
func decodeUint(r result3) (*big.Int, bool) {
	if !r.Success || len(r.ReturnData) == 0 {
		return nil, false
	}
	return new(big.Int).SetBytes(r.ReturnData), true
}
