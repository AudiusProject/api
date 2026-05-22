// Standalone verification of the eth-indexer's contract-read path.
// Run with:
//
//	go run ./cmd/eth_smoke <holder-address>
//
// Reads balanceOf + totalStakedFor + getTotalDelegatorStake against the
// configured Alchemy endpoint and prints each component plus the sum.
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	audioContract   = "0x18aAA7115705e8be94bfFEbDE57Af9BFc265B998"
	stakingContract = "0xe6D97B2099F142513be7A2a068bE040656Ae4591"
	delegateManager = "0x4d7968ebfD390D5E7926Cb3587C39eFf2F9FB225"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: eth_smoke <holder-address>")
	}
	rpcURL := os.Getenv("ethRpcUrl")
	if rpcURL == "" {
		log.Fatal("ethRpcUrl env var must be set")
	}
	holder := common.HexToAddress(os.Args[1])

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer client.Close()

	call := func(contract common.Address, sig string) (*big.Int, error) {
		sel := crypto.Keccak256([]byte(sig))[:4]
		data := append(sel, common.LeftPadBytes(holder.Bytes(), 32)...)
		out, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sig, err)
		}
		if len(out) == 0 {
			return big.NewInt(0), nil
		}
		return new(big.Int).SetBytes(out), nil
	}

	b, err := call(common.HexToAddress(audioContract), "balanceOf(address)")
	if err != nil {
		log.Fatal(err)
	}
	s, err := call(common.HexToAddress(stakingContract), "totalStakedFor(address)")
	if err != nil {
		log.Fatal(err)
	}
	d, err := call(common.HexToAddress(delegateManager), "getTotalDelegatorStake(address)")
	if err != nil {
		log.Fatal(err)
	}

	total := new(big.Int).Add(b, s)
	total.Add(total, d)

	fmt.Printf("holder:                 %s\n", holder.Hex())
	fmt.Printf("balanceOf:              %s wei\n", b)
	fmt.Printf("totalStakedFor:         %s wei\n", s)
	fmt.Printf("getTotalDelegatorStake: %s wei\n", d)
	fmt.Printf("total:                  %s wei\n", total)
}
