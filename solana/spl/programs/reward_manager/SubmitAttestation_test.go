package reward_manager_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
)

func TestDecodeSubmitAttestationsInstruction(t *testing.T) {
	// Real tx: 2JDkdz3uUzmAa7yzccdNz2nCbTxmhoxab1SBV4JrKGvPWqmM3txvffwvQfJBPMSWZsyjGy5vnCrYwUEhqSCmRdwa
	tx, _ := solana.TransactionFromBase64("AUDd4/CTZHmUgxb6eW+28W9y+H9UkDyBxA0Ssxq5zFWw+w3sGn/0VbmInZ/GaaKJgoVMCAL0/rgUJb2++Qf1AA2AAQADBQT8D+wyuUlpRD076vzPBw7SbQCcoUnIL4FAO1Cheug8tM4xwWCJGPcAbHZSSTxu3ZW1lAGZ+ZZ3EJ3qhIbOmi8Exvwg8FDM8FWE1yEcn4z1nsFHhbsWah4oMOgSIAAAAKa5zF7XPISSa5HZUS18kOPLVacuyslBg2R0b/CnEetTAwZGb+UhFzL/7K26csOb57yM5bvF9xJrLEObOkAAAAC4db3+CiRsBTmNCFUcD5KQ9RaET00zlvkCJba7Xc+u0AoCAJABASAAAAwAAGEALwAAALZGLpVdpYQbbZ4eJSm4MPAPMb86MqKVx89NjWvrUbkZXl9Gw7/NlWuirIDTmJ96Q+3Q1X6KQIK6Ez05bGgE8MT1ggF7pp+Y7Dkl134nl3WC+di4AGddosBKBjXGf9WCrR6BLgr2EujzXwDh9QUAAAAAX2M6MzIzZWE0YTA6MjAyNTI1AwgBBQYABwgJChYGEQAAAGM6MzIzZWE0YTA6MjAyNTI1AgClAQEgAAIMAAJhAEQAAoMR9ZtyUi5ygjHcYCJjWaUYePmhoFC8JpLRn5u434YMgdvdQfap5nyAzZeJGji9RrGZPFhFgxNhA+udAHgZGYXGZ8vShAFJUwMijbjaLbdoO0eacAFnXaLASgY1xn/Vgq0egS4K9hLo818A4fUFAAAAAF9jOjMyM2VhNGEwOjIwMjUyNV8AtkYulV2lhBttnh4lKbgw8A8xvwMIAQUGAAsICQoWBhEAAABjOjMyM2VhNGEwOjIwMjUyNQIApQEBIAAEDAAEYQBEAARemMvuqirO3sCDOsPRY04qeuDzwsCnAPn0wBF3aobRPXtXVYpqYjQk7+9npC0Hib5yxASzIdWaNH0a98zq3fjx/UlzzUne5w0PKllnCn3zHCLq+AwBZ12iwEoGNcZ/1YKtHoEuCvYS6PNfAOH1BQAAAABfYzozMjNlYTRhMDoyMDI1MjVfALZGLpVdpYQbbZ4eJSm4MPAPMb8DCAEFBgAMCAkKFgYRAAAAYzozMjNlYTRhMDoyMDI1MjUCAKUBASAABgwABmEARAAGj8+hC9OAhXCYfbtbHvSrdEAPv9rrsGvLYXRLJhHxHGvEM0zvdYHWbYvVTJdCAOpL3Mwnkh6UTFXSH2CKl1At9f/2jjzBjVj0zp02OUz7Y3A2c+1yAGddosBKBjXGf9WCrR6BLgr2EujzXwDh9QUAAAAAX2M6MzIzZWE0YTA6MjAyNTI1XwC2Ri6VXaWEG22eHiUpuDDwDzG/AwgBBQYADQgJChYGEQAAAGM6MzIzZWE0YTA6MjAyNTI1BAAJA/BJAgAAAAAABAAFAqtQAQABrb/XxHksdnv9qTHAMEjJ/ohseEmFUcuv1+4iZlgN6u8ACQUGCAECAA4MCw==")
	tx.Message.SetAddressTables(stagingLookupTables)

	expectedAttestations := "DAnkCx6M9Q2bPaMGkFwozrps6m9xggRWQMe5qCcpo7CS"
	expectedState := "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"
	expectedAuthority := "6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa"
	expectedSender := "FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk"
	expectedPayer := "LTZEyCYzn5pRLtTg6PCZkVCo8msj5sXTYP65TGWqgaP"
	expectedDisbursementId := "c:323ea4a0:202525"

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[1]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	decoded, err := reward_manager.DecodeInstruction(accounts, compiled.Data)
	require.NoError(t, err)
	inst, ok := decoded.Impl.(*reward_manager.SubmitAttestation)
	if !ok {
		assert.Fail(t, "bad type assert")
	}
	assert.Equal(t, expectedAttestations, inst.AttestationsAccount().PublicKey.String())
	assert.Equal(t, expectedState, inst.RewardManagerStateAccount().PublicKey.String())
	assert.Equal(t, expectedAuthority, inst.AuthorityAccount().PublicKey.String())
	assert.Equal(t, expectedSender, inst.SenderAccount().PublicKey.String())
	assert.Equal(t, expectedPayer, inst.PayerAccount().PublicKey.String())

	assert.Equal(t, expectedDisbursementId, inst.DisbursementId)

	// Tests the EncodeToTree serialization
	buf := new(bytes.Buffer)
	encoder := text.NewTreeEncoder(buf, "")
	decoded.EncodeToTree(encoder)
	_, err = encoder.WriteString(encoder.Tree.String())
	require.NoError(t, err)
	s := buf.String()

	assert.Contains(t, s, expectedAttestations)
	assert.Contains(t, s, expectedState)
	assert.Contains(t, s, expectedAuthority)
	assert.Contains(t, s, expectedSender)
	assert.Contains(t, s, expectedPayer)
	assert.Contains(t, s, expectedDisbursementId)
}

func TestBuildSubmitAttestationsInstruction(t *testing.T) {
	// Test Data
	challengeId := "c"
	specifier := "323ea4a0:202525"
	senderEthAddress := common.HexToAddress("0x00b6462e955dA5841b6D9e1E2529B830F00f31Bf")

	// Expectations
	expectedAttestations := "DAnkCx6M9Q2bPaMGkFwozrps6m9xggRWQMe5qCcpo7CS"
	expectedState := "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"
	expectedAuthority := "6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa"
	expectedSender := "FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk"
	expectedPayer := "LTZEyCYzn5pRLtTg6PCZkVCo8msj5sXTYP65TGWqgaP"
	expectedData, _ := hex.DecodeString("0611000000633a33323365613461303a323032353235")

	// Use stage program ID
	stageProgramId := solana.MustPublicKeyFromBase58("CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp")
	reward_manager.SetProgramID(stageProgramId)

	// Test happy path
	{
		instBuilder, err := reward_manager.NewSubmitAttestationInstruction(
			challengeId,
			specifier,
			senderEthAddress,
			solana.MustPublicKeyFromBase58(expectedState),
			solana.MustPublicKeyFromBase58(expectedPayer),
		)
		require.NoError(t, err)

		inst := instBuilder.Build()
		require.NoError(t, err)

		accounts := inst.Accounts()
		assert.Equal(t, stageProgramId, inst.ProgramID())
		assert.Len(t, accounts, 8)
		assert.Equal(t, expectedAttestations, accounts[0].PublicKey.String())
		assert.False(t, accounts[0].IsSigner)
		assert.True(t, accounts[0].IsWritable)
		assert.Equal(t, expectedState, accounts[1].PublicKey.String())
		assert.False(t, accounts[1].IsSigner)
		assert.False(t, accounts[1].IsWritable)
		assert.Equal(t, expectedAuthority, accounts[2].PublicKey.String())
		assert.False(t, accounts[2].IsSigner)
		assert.False(t, accounts[2].IsWritable)
		assert.Equal(t, expectedPayer, accounts[3].PublicKey.String())
		assert.True(t, accounts[3].IsSigner)
		assert.True(t, accounts[3].IsWritable)
		assert.Equal(t, expectedSender, accounts[4].PublicKey.String())
		assert.False(t, accounts[4].IsSigner)
		assert.False(t, accounts[4].IsWritable)

		data, err := inst.Data()
		require.NoError(t, err)
		assert.Equal(t, expectedData, data)
	}

	// Test validation
	{
		dummyDisbursementId := "xx:yy"
		dummyPubKey := solana.MustPublicKeyFromBase58(expectedPayer)

		// Missing disbursementId
		err := reward_manager.NewSubmitAttestationInstructionBuilder().
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			SetSenderAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing attestationsAccount
		err = reward_manager.NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			SetSenderAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing rewardManagerStateAccount
		err = reward_manager.NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetAttestationsAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			SetSenderAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing authorityAccount
		err = reward_manager.NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			SetSenderAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing payerAccount
		err = reward_manager.NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetSenderAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing senderAccount
		_, err = reward_manager.NewSubmitAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			ValidateAndBuild()
		assert.Error(t, err)
	}
}
