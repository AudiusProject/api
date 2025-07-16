package reward_manager_test

import (
	"testing"

	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

var stagingLookupTables = map[solana.PublicKey]solana.PublicKeySlice{
	solana.MustPublicKeyFromBase58("ChFCWjeFxM6SRySTfT46zXn2K7m89TJsft4HWzEtkB4J"): solana.PublicKeySlice{
		solana.SystemProgramID,
		solana.SysVarRentPubkey,
		solana.SysVarInstructionsPubkey,
		solana.TokenProgramID,
		solana.Token2022ProgramID,
		solana.MustPublicKeyFromBase58("GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"),
		solana.MustPublicKeyFromBase58("6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa"),
		solana.MustPublicKeyFromBase58("HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ"),
		solana.MustPublicKeyFromBase58("FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk"),
		solana.MustPublicKeyFromBase58("4UVoSQFvfvJLo6wvzyDZ7GJA3gmBqQ9aVSU8rdyPts6R"),
		solana.MustPublicKeyFromBase58("68Yjq1wfh8Vf4BCNFakAqWryPTWdcrdn1kprNHHmmk4p"),
		solana.MustPublicKeyFromBase58("HfBGwQHpXmfsVVFYzjjyHE7vrqsFdvimTbsNpSmVZCou"),
		solana.MustPublicKeyFromBase58("Awd1sHgpc6TAeeSyoSYwW6XYyfKgDRUPzTDPJePJufLF"),
		solana.MustPublicKeyFromBase58("7DEE1bWzGYMK8DG8ZhBkXmqe7Lu5HqTpfB1ene8sr7St"),
		solana.MustPublicKeyFromBase58("2GBs4JCWZ7kfZfDeejSXj35Xr6Ytor3K8JQmwi26kWQ5"),
		solana.MustPublicKeyFromBase58("6Cxzxt3paPPsBevxHuhy5rZr4adedcHqR2d5xztqMLZf"),
		solana.MustPublicKeyFromBase58("8XX21N3aqBcUxTEDpGqKyFZcZn4iHbMii52dWwagXtDL"),
		solana.MustPublicKeyFromBase58("9kfSAxU6LruhXJbCNdp3APohHj4epmyB7eNK9HeSv9SR"),
		solana.MustPublicKeyFromBase58("GHsYnWhkgztGX6GK4GDkVTuyDtKRBDcWFqP6RwEKasPS"),
		solana.MustPublicKeyFromBase58("AB6r7n6CS1jK3sgu4zrc7N4kbqdtQXuxd5FeMmDRb2oq"),
		solana.MustPublicKeyFromBase58("5XTwQHNdLvQjnixx7wxfcnL98kXqSsMSPbS9YPGyRXyJ"),
	},
}

func TestDecodeInstruction(t *testing.T) {
	// Real tx: 4iCX6Au7yfVrUSZ1wtEEDiwA1fXrXScVDLQymNPgD5B8hJ5zaecJKteDxZm8ZiKwFC39QPPma5QxA1MPeecfDzEu
	tx, _ := solana.TransactionFromBase64("AbmUQCJ3cqX8kzxlqFaVjFVCy0LRNTMH/N+pf3BG0+GvbWKA0BId1o8C8+1w5Hey0gnJUdF29n+TCrmZVTr18QqAAQACBgT8D+wyuUlpRD076vzPBw7SbQCcoUnIL4FAO1Cheug8tM4xwWCJGPcAbHZSSTxu3ZW1lAGZ+ZZ3EJ3qhIbOmi+KU9u7lVtr4y8SQZGMrj+TM+7+vHMCC2Yn6JF02t6mZpKtWHxI0iSbGVs0UaMrTIaDXpNEXS0kiGcZOjDdvWaeprnMXtc8hJJrkdlRLXyQ48tVpy7KyUGDZHRv8KcR61MDBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAANn180M7cF8TbhIPejJ/Xmwz9/w7Xjpp0csYl8AJuHcbAwQLAQcIBgIDCQAKCwwyBwDh9QUAAAAAEQAAAGM6MzIzZWE0YTA6MjAyNTI1Z12iwEoGNcZ/1YKtHoEuCvYS6PMFAAkD8EkCAAAAAAAFAAUCYcYAAAGtv9fEeSx2e/2pMcAwSMn+iGx4SYVRy6/X7iJmWA3q7wEHBgUGCAEDAA==")
	tx.Message.SetAddressTables(stagingLookupTables)

	originalProgramId := solana.MustPublicKeyFromBase58("DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei")
	expectedProgramId := solana.MustPublicKeyFromBase58("CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp")

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[0]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	before, err := solana.DecodeInstruction(originalProgramId, accounts, compiled.Data)
	require.NoError(t, err)
	reward_manager.SetProgramID(expectedProgramId)
	after, err := solana.DecodeInstruction(expectedProgramId, accounts, compiled.Data)
	require.NoError(t, err)

	assert.Equal(t, before, after)

	reward_manager.SetProgramID(originalProgramId)

}
