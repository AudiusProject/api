package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func uint256Bytes(n int64) []byte {
	return common.LeftPadBytes(big.NewInt(n).Bytes(), 32)
}

func TestDecodeUint(t *testing.T) {
	tests := []struct {
		name    string
		r       result3
		wantOK  bool
		wantVal int64
	}{
		{
			name:    "success with 32-byte uint256",
			r:       result3{Success: true, ReturnData: uint256Bytes(123)},
			wantOK:  true,
			wantVal: 123,
		},
		{
			name:    "success with zero",
			r:       result3{Success: true, ReturnData: uint256Bytes(0)},
			wantOK:  true,
			wantVal: 0,
		},
		{
			name:   "failed call short-circuits",
			r:      result3{Success: false, ReturnData: uint256Bytes(999)},
			wantOK: false,
		},
		{
			name:   "success but empty data",
			r:      result3{Success: true, ReturnData: nil},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeUint(tt.r)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.NotNil(t, got)
				assert.Equal(t, big.NewInt(tt.wantVal).String(), got.String())
			}
		})
	}
}

// TestMulticallEncodingRoundtrip exercises the ABI pack/unpack path that
// the live multicall depends on, without touching the network. Catches
// regressions in:
//   - Call3[] / Result3[] tuple type definitions (field names must match
//     our call3 / result3 structs for the reflection-based encode/decode
//     to work)
//   - The abi.ConvertType coercion from anonymous-struct slice back into
//     our named type — go-ethereum's behavior here is the subtle part
func TestMulticallEncodingRoundtrip(t *testing.T) {
	calls := []call3{
		{
			Target:       common.HexToAddress("0xdead000000000000000000000000000000000001"),
			AllowFailure: true,
			CallData:     []byte{0x70, 0xa0, 0x82, 0x31, 0xaa, 0xbb},
		},
		{
			Target:       common.HexToAddress("0xbeef000000000000000000000000000000000002"),
			AllowFailure: false,
			CallData:     []byte{0x4b, 0x34, 0x1a, 0xed},
		},
	}

	// Pack Call3[]
	encoded, err := multicall3Args.Pack(calls)
	require.NoError(t, err, "packing Call3[] failed")
	require.NotEmpty(t, encoded)

	// And confirm Call3[] roundtrips back cleanly (this catches any drift
	// between our `call3` struct field names and the ABI tuple component
	// names).
	decodedCalls, err := multicall3Args.Unpack(encoded)
	require.NoError(t, err, "unpacking Call3[] failed")
	require.Len(t, decodedCalls, 1)
	roundtripped := *abi.ConvertType(decodedCalls[0], new([]call3)).(*[]call3)
	require.Len(t, roundtripped, len(calls))
	for i, c := range calls {
		assert.Equal(t, c.Target, roundtripped[i].Target, "call[%d].Target", i)
		assert.Equal(t, c.AllowFailure, roundtripped[i].AllowFailure, "call[%d].AllowFailure", i)
		assert.Equal(t, c.CallData, roundtripped[i].CallData, "call[%d].CallData", i)
	}

	// Now exercise the Result3[] path the way live multicall responses
	// flow through it: pack a synthetic results blob (one success with a
	// uint256 return, one failure with empty return data), then unpack +
	// ConvertType, then verify decodeUint reads each correctly.
	results := []result3{
		{Success: true, ReturnData: uint256Bytes(42)},
		{Success: false, ReturnData: nil},
	}
	resultBytes, err := multicall3Result.Pack(results)
	require.NoError(t, err)

	decodedResults, err := multicall3Result.Unpack(resultBytes)
	require.NoError(t, err)
	require.Len(t, decodedResults, 1)
	coerced := *abi.ConvertType(decodedResults[0], new([]result3)).(*[]result3)
	require.Len(t, coerced, 2)

	v0, ok := decodeUint(coerced[0])
	require.True(t, ok)
	assert.Equal(t, "42", v0.String())

	_, ok = decodeUint(coerced[1])
	assert.False(t, ok, "failure result should not decode")
}

// TestAggregate3Selector pins the function selector we send on every
// multicall against the canonical signature. If go-ethereum's keccak ever
// drifts or somebody edits the selector helper, this fails loudly instead
// of silently sending calls to a no-op selector on the Multicall3
// contract (which would just revert).
func TestAggregate3Selector(t *testing.T) {
	// aggregate3((address,bool,bytes)[]) -> keccak256 first 4 bytes
	expected := []byte{0x82, 0xad, 0x56, 0xcb}
	assert.Equal(t, expected, aggregate3Selector,
		"aggregate3 selector drifted — Multicall3 will not route this call")
}
