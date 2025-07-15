package reward_manager_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
)

func TestDecodeEvaluateAttestationsInstruction(t *testing.T) {
	// Real tx: 4iCX6Au7yfVrUSZ1wtEEDiwA1fXrXScVDLQymNPgD5B8hJ5zaecJKteDxZm8ZiKwFC39QPPma5QxA1MPeecfDzEu
	tx, _ := solana.TransactionFromBase64("AbmUQCJ3cqX8kzxlqFaVjFVCy0LRNTMH/N+pf3BG0+GvbWKA0BId1o8C8+1w5Hey0gnJUdF29n+TCrmZVTr18QqAAQACBgT8D+wyuUlpRD076vzPBw7SbQCcoUnIL4FAO1Cheug8tM4xwWCJGPcAbHZSSTxu3ZW1lAGZ+ZZ3EJ3qhIbOmi+KU9u7lVtr4y8SQZGMrj+TM+7+vHMCC2Yn6JF02t6mZpKtWHxI0iSbGVs0UaMrTIaDXpNEXS0kiGcZOjDdvWaeprnMXtc8hJJrkdlRLXyQ48tVpy7KyUGDZHRv8KcR61MDBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAANn180M7cF8TbhIPejJ/Xmwz9/w7Xjpp0csYl8AJuHcbAwQLAQcIBgIDCQAKCwwyBwDh9QUAAAAAEQAAAGM6MzIzZWE0YTA6MjAyNTI1Z12iwEoGNcZ/1YKtHoEuCvYS6PMFAAkD8EkCAAAAAAAFAAUCYcYAAAGtv9fEeSx2e/2pMcAwSMn+iGx4SYVRy6/X7iJmWA3q7wEHBgUGCAEDAA==")
	tx.Message.SetAddressTables(stagingLookupTables)

	expectedAttestations := "DAnkCx6M9Q2bPaMGkFwozrps6m9xggRWQMe5qCcpo7CS"
	expectedState := "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"
	expectedAuthority := "6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa"
	expectedTokenSource := "HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ"
	expectedDestination := "AJyRmWXxk25UBUqVi4FX4bkCgXHBfRjNQdNusXzHkPeM"
	expectedDisbursement := "AsZqEeFqoPR2LEnBb4P1XG5YK8u24kFAoSSUKot8AE3X"
	expectedAntiAbuseOracle := "FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk"
	expectedPayer := "LTZEyCYzn5pRLtTg6PCZkVCo8msj5sXTYP65TGWqgaP"
	expectedEthAddress := "0x675dA2C04a0635c67fD582ad1e812E0Af612E8f3"
	expectedAmount := 100000000
	expectedDisbursementId := "c:323ea4a0:202525"

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[0]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	decoded, err := reward_manager.DecodeInstruction(accounts, compiled.Data)
	require.NoError(t, err)
	inst, ok := decoded.Impl.(*reward_manager.EvaluateAttestation)
	if !ok {
		assert.Fail(t, "bad type assert")
	}
	assert.Equal(t, expectedAttestations, inst.AttestationsAccount().PublicKey.String())
	assert.Equal(t, expectedState, inst.RewardManagerStateAccount().PublicKey.String())
	assert.Equal(t, expectedAuthority, inst.AuthorityAccount().PublicKey.String())
	assert.Equal(t, expectedTokenSource, inst.TokenSourceAccount().PublicKey.String())
	assert.Equal(t, expectedDestination, inst.DestinationUserBankAccount().PublicKey.String())
	assert.Equal(t, expectedDisbursement, inst.DisbursementAccount().PublicKey.String())
	assert.Equal(t, expectedAntiAbuseOracle, inst.AntiAbuseOracleAccount().PublicKey.String())
	assert.Equal(t, expectedPayer, inst.PayerAccount().PublicKey.String())

	assert.Equal(t, expectedAmount, int(inst.Amount))
	assert.Equal(t, expectedDisbursementId, inst.DisbursementId)
	assert.Equal(t, expectedEthAddress, inst.RecipientEthAddress.String())

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
	assert.Contains(t, s, expectedTokenSource)
	assert.Contains(t, s, expectedDestination)
	assert.Contains(t, s, expectedDisbursement)
	assert.Contains(t, s, expectedAntiAbuseOracle)
	assert.Contains(t, s, expectedPayer)
	assert.Contains(t, s, fmt.Sprintf("%d", expectedAmount))
	assert.Contains(t, s, expectedDisbursementId)
	assert.Contains(t, s, expectedEthAddress)
}

func TestBuildEvaluateAttestationsInstruction(t *testing.T) {
	// Test data
	challengeId := "ft"
	specifier := "37364e80"
	recipientEthAddress := common.HexToAddress("0x3f6d9fcf0d4466dd5886e3b1def017adfb7916b4")
	amount := uint64(200000000)
	antiAbuseOracleEthAddress := common.HexToAddress("0x00b6462e955dA5841b6D9e1E2529B830F00f31Bf")

	// Expected Accounts
	// Real tx: 26gT9HVMhzBDzsKcsiKREYmGcXuZhjAJpCVUu9WFNhVMyKje8SdApYc4ev3HrumZB4LEXLUaPnKyriBPLmtzwrWp)
	rewardState := "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"
	expectedAuthority := "6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa"
	tokenSource := "HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ"
	destinationUserBank := "Cjv8dvVfWU8wUYAR82T5oZ4nHLB6EyGNvpPBzw3r76Qy"
	expectedDisbursement := "3qQfuDEBWEmxRo5G4J2a4eYUVf9u1LWzLgRPndiwew2w"
	expectedOracle := "FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk"
	payer := "E3CfijtAJwBSHfwFEViAUd3xp7c8TBxwC1eXn1Fgxp8h"

	// Expected Data (from same tx)
	expectedData, err := hex.DecodeString("0700c2eb0b000000000b00000066743a33373336346538303f6d9fcf0d4466dd5886e3b1def017adfb7916b4")
	require.NoError(t, err)

	// Use stage program ID
	stageProgramId := solana.MustPublicKeyFromBase58("CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp")
	reward_manager.SetProgramID(stageProgramId)

	// Test happy path
	{
		instBuilder, err := reward_manager.NewEvaluateAttestationInstruction(
			challengeId,
			specifier,
			recipientEthAddress,
			amount,
			antiAbuseOracleEthAddress,
			solana.MustPublicKeyFromBase58(rewardState),
			solana.MustPublicKeyFromBase58(tokenSource),
			solana.MustPublicKeyFromBase58(destinationUserBank),
			solana.MustPublicKeyFromBase58(payer),
		)
		require.NoError(t, err)

		inst, err := instBuilder.ValidateAndBuild()
		require.NoError(t, err)

		accounts := inst.Accounts()
		assert.Equal(t, stageProgramId, inst.ProgramID())
		assert.Len(t, accounts, 11)
		assert.Equal(t, rewardState, accounts[1].PublicKey.String())
		assert.Equal(t, expectedAuthority, accounts[2].PublicKey.String())
		assert.Equal(t, tokenSource, accounts[3].PublicKey.String())
		assert.Equal(t, destinationUserBank, accounts[4].PublicKey.String())
		assert.Equal(t, expectedDisbursement, accounts[5].PublicKey.String())
		assert.Equal(t, expectedOracle, accounts[6].PublicKey.String())
		assert.Equal(t, payer, accounts[7].PublicKey.String())

		data, err := inst.Data()
		require.NoError(t, err)
		assert.Equal(t, expectedData, data)
	}
	// Test validation
	{
		dummyDisbursementId := "xx:yy"
		dummyEth := recipientEthAddress
		dummyPubKey := solana.MustPublicKeyFromBase58(payer)

		// Missing amount
		err := reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing disbursementId
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing recipientEthAddress
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing attestations
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing state
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing authority
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing token source
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing destination
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing disbursement
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing antiAbuseOracle
		err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetPayerAccount(dummyPubKey).
			Validate()
		assert.Error(t, err)

		// Missing payer
		_, err = reward_manager.NewEvaluateAttestationInstructionBuilder().
			SetAmount(amount).
			SetDisbursementId(dummyDisbursementId).
			SetRecipientEthAddress(dummyEth).
			SetAttestationsAccount(dummyPubKey).
			SetRewardManagerStateAccount(dummyPubKey).
			SetAuthorityAccount(dummyPubKey).
			SetTokenSourceAccount(dummyPubKey).
			SetDestinationUserBankAccount(dummyPubKey).
			SetDisbursementAccount(dummyPubKey).
			SetAntiAbuseOracleAccount(dummyPubKey).
			ValidateAndBuild()
		assert.Error(t, err)
	}
}
