package indexers

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// Maps a Geyser pb.TransactionStatusMeta to a solana-go rpc.TransactionMeta
func toMeta(meta *pb.TransactionStatusMeta) (*rpc.TransactionMeta, error) {
	innerInstructions := make([]rpc.InnerInstruction, len(meta.InnerInstructions))
	for i, inst := range meta.InnerInstructions {
		innerInsts := make([]rpc.CompiledInstruction, len(inst.Instructions))
		for j, innerInst := range inst.Instructions {
			accounts := make([]uint16, len(innerInst.Accounts))
			for k, acc := range innerInst.Accounts {
				accounts[k] = uint16(acc)
			}
			innerInsts[j] = rpc.CompiledInstruction{
				Accounts:       accounts,
				Data:           innerInst.Data,
				ProgramIDIndex: uint16(innerInst.ProgramIdIndex),
				StackHeight:    uint16(*innerInst.StackHeight),
			}
		}
		innerInstructions[i] = rpc.InnerInstruction{
			Index:        uint16(inst.Index),
			Instructions: innerInsts,
		}
	}
	preTokenBalances := make([]rpc.TokenBalance, len(meta.PreTokenBalances))
	for i, bal := range meta.PreTokenBalances {
		mint, err := solana.PublicKeyFromBase58(bal.Mint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mint: %w", err)
		}
		programId, err := solana.PublicKeyFromBase58(bal.ProgramId)
		if err != nil {
			return nil, fmt.Errorf("failed to parse programId: %w", err)
		}
		owner, err := solana.PublicKeyFromBase58(bal.Owner)
		if err != nil {
			return nil, fmt.Errorf("failed to parse owner: %w", err)
		}
		preTokenBalances[i] = rpc.TokenBalance{
			Owner:        &owner,
			ProgramId:    &programId,
			AccountIndex: uint16(bal.AccountIndex),
			Mint:         mint,
			UiTokenAmount: &rpc.UiTokenAmount{
				Amount:         bal.UiTokenAmount.Amount,
				Decimals:       uint8(bal.UiTokenAmount.Decimals),
				UiAmount:       &bal.UiTokenAmount.UiAmount,
				UiAmountString: bal.UiTokenAmount.UiAmountString,
			},
		}
	}
	postTokenBalances := make([]rpc.TokenBalance, len(meta.PostTokenBalances))
	for i, bal := range meta.PostTokenBalances {
		mint, err := solana.PublicKeyFromBase58(bal.Mint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mint: %w", err)
		}
		postTokenBalances[i] = rpc.TokenBalance{
			AccountIndex: uint16(bal.AccountIndex),
			Mint:         mint,
			UiTokenAmount: &rpc.UiTokenAmount{
				Amount:         bal.UiTokenAmount.Amount,
				Decimals:       uint8(bal.UiTokenAmount.Decimals),
				UiAmount:       &bal.UiTokenAmount.UiAmount,
				UiAmountString: bal.UiTokenAmount.UiAmountString,
			},
		}
	}
	rewards := make([]rpc.BlockReward, len(meta.Rewards))
	for i, reward := range meta.Rewards {
		pubkey, err := solana.PublicKeyFromBase58(reward.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reward: %w", err)
		}
		var commission *uint8
		parsed, err := strconv.ParseUint(reward.Commission, 10, 8)
		if err == nil {
			val := uint8(parsed)
			commission = &val
		}
		rewards[i] = rpc.BlockReward{
			Pubkey:      pubkey,
			Lamports:    reward.Lamports,
			PostBalance: reward.PostBalance,
			RewardType:  rpc.RewardType(reward.RewardType.String()),
			Commission:  commission,
		}
	}
	loadedReadonlyAddresses := make([]solana.PublicKey, len(meta.LoadedReadonlyAddresses))
	for i, addr := range meta.LoadedReadonlyAddresses {
		loadedReadonlyAddresses[i] = solana.PublicKeyFromBytes(addr)
	}
	loadedWritableAddresses := make([]solana.PublicKey, len(meta.LoadedWritableAddresses))
	for i, addr := range meta.LoadedWritableAddresses {
		loadedWritableAddresses[i] = solana.PublicKeyFromBytes(addr)
	}
	returnData := rpc.ReturnData{}
	if meta.ReturnData != nil {
		var returnDataData solana.Data
		err := json.Unmarshal(meta.ReturnData.Data, &returnDataData)
		if err != nil {
			return nil, fmt.Errorf("failed to return data: %w", err)
		}
		returnData.ProgramId = solana.PublicKeyFromBytes(meta.ReturnData.ProgramId)
		returnData.Data = returnDataData
	}
	res := rpc.TransactionMeta{
		Err:               meta.Err,
		Fee:               meta.Fee,
		PreBalances:       meta.PreBalances,
		PostBalances:      meta.PostBalances,
		InnerInstructions: innerInstructions,
		PreTokenBalances:  preTokenBalances,
		PostTokenBalances: postTokenBalances,
		LogMessages:       meta.LogMessages,
		Rewards:           rewards,
		LoadedAddresses: rpc.LoadedAddresses{
			ReadOnly: loadedReadonlyAddresses,
			Writable: loadedWritableAddresses,
		},
		ReturnData:           returnData,
		ComputeUnitsConsumed: meta.ComputeUnitsConsumed,
	}
	return &res, nil
}

// Maps a Geyser pb.Transaction to a solana-go solana.Transaction
func toTransaction(tx *pb.Transaction) *solana.Transaction {
	signatures := make([]solana.Signature, len(tx.Signatures))
	for i, sig := range tx.Signatures {
		signatures[i] = solana.Signature(sig)
	}
	accountKeys := make([]solana.PublicKey, len(tx.Message.AccountKeys))
	for i, acc := range tx.Message.AccountKeys {
		accountKeys[i] = solana.PublicKeyFromBytes(acc)
	}
	instructions := make([]solana.CompiledInstruction, len(tx.Message.Instructions))
	for i, inst := range tx.Message.Instructions {
		accounts := make([]uint16, len(inst.Accounts))
		for j, acc := range inst.Accounts {
			accounts[j] = uint16(acc)
		}
		instructions[i] = solana.CompiledInstruction{
			ProgramIDIndex: uint16(inst.ProgramIdIndex),
			Accounts:       accounts,
			Data:           inst.Data,
		}
	}
	addressTableLookups := make([]solana.MessageAddressTableLookup, len(tx.Message.AddressTableLookups))
	for i, lu := range tx.Message.AddressTableLookups {
		addressTableLookups[i] = solana.MessageAddressTableLookup{
			AccountKey:      solana.PublicKey(lu.AccountKey),
			WritableIndexes: lu.WritableIndexes,
			ReadonlyIndexes: lu.ReadonlyIndexes,
		}
	}
	res := solana.Transaction{
		Signatures: signatures,
		Message: solana.Message{
			AccountKeys: accountKeys,
			Header: solana.MessageHeader{
				NumRequiredSignatures:       uint8(tx.Message.Header.NumRequiredSignatures),
				NumReadonlySignedAccounts:   uint8(tx.Message.Header.NumReadonlySignedAccounts),
				NumReadonlyUnsignedAccounts: uint8(tx.Message.Header.NumReadonlyUnsignedAccounts),
			},
			RecentBlockhash:     solana.Hash(tx.Message.RecentBlockhash),
			Instructions:        instructions,
			AddressTableLookups: addressTableLookups,
		},
	}
	return &res
}
