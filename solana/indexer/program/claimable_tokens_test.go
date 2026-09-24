package program

import (
	"testing"

	"api.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/require"
	"go.uber.org/zap"
)

func TestProcessClaimableTokensSetAuthority(t *testing.T) {
	accountKeys := solana.PublicKeySlice{
		solana.MustPublicKeyFromBase58("8teuqGNB7RhwC2JoDT961J3jStBfXeu4nRxyubu36Kpe"),
		solana.MustPublicKeyFromBase58("5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx"),
		solana.SysVarInstructionsPubkey,
		solana.SysVarRecentBlockHashesPubkey,
		solana.TokenProgramID,
		claimable_tokens.ProgramID,
	}
	instruction := solana.CompiledInstruction{
		ProgramIDIndex: 5,
		Accounts:       []uint16{0, 1, 2, 3, 4},
		Data:           solana.Base58{claimable_tokens.Instruction_SetAuthority},
	}
	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: accountKeys,
			Header: solana.MessageHeader{
				NumReadonlyUnsignedAccounts: 5,
			},
			Instructions: []solana.CompiledInstruction{{}, instruction},
		},
	}

	err := processClaimableTokensInstruction(
		t.Context(),
		nil,
		449737686,
		tx,
		1,
		instruction,
		"2RuRg4MCAJHF5dQifhGzkZ6qD7XQGbGop4Z48fxgweHawi5FNKiwiTVYwfQ7hzgU1MgJkjaKV7SWumr8ztGa98cf",
		zap.NewNop(),
	)
	require.NoError(t, err)
}
