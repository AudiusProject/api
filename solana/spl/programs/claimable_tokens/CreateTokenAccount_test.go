package claimable_tokens_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/text"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func TestDecodeCreateInstruction(t *testing.T) {
	// Real tx: 5e14bb3amrDFh4NxwJkGb6aCcSZBekksfcKyLJ2EJzwUtocPdaT1Ymnt7okhBAMpapW9LTeE4Kv3XqjGRrjmQaaN
	tx, _ := solana.TransactionFromBase64("Aef574QaY0svg2Or3pYRpRKwjIKnSC1O7C4LuY3oMxR7xm8UBzno7+oCKxXQO+42MSvogbPwSW2UOsCENCskLwuAAQAHCX20n9GpBXG3zC0dBBSzqvyiyw7iAyLnv1nExZzn6pXSAsjVqYHcbgzQouttHgyoobHWpPor/j0GZtd5lyoKGerPLvKviFfPDcCe1kbtn+YIorUsNzcMKO+I3ElSvGIHzXv8M8wudcFIdsw3koOQT0kHdmkRIle7Ot/zaYtgIklAQ8/yZUFBaDbkjfDoL78d9G9SXl8sqYoQJMJGCCiR5mUGp9UXGSxcUSGMyUw9SvF/WNruCJuh/UTj29mKAAAAAAbd9uHXZaGT2cvhRs7reawctIXtX1s3kTqM9YV+/wCpAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAAP9HWb4mCsexwNIodHbyqw2+gQx/y/PbFWyTjUgGxiRGAwIHAAMEAQUGBxUAWX2u4XzoQLzu1+A/NyBX106nrdoIAAkD8EkCAAAAAAAIAAUCRZYAAAA=")

	expectedPayer := "9Thi2E1AfAqB3C7ZaUuLVV9QYM8UtPKumYwEBmcEUaB3"
	expectedMint := "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
	expectedAuthority := "5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx"
	expectedUserBank := "BsSCpqCamyBv3c9wyTFmZzmJmHtPpkw9ijC5whPQQFB"
	expectedEthAddress := "0x597daee17ce840bceed7e03f372057d74Ea7addA"

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[0]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	decoded, err := claimable_tokens.DecodeInstruction(accounts, compiled.Data)
	require.NoError(t, err)
	inst, ok := decoded.Impl.(*claimable_tokens.CreateTokenAccount)
	if !ok {
		assert.Fail(t, "bad type assert")
	}
	assert.Equal(t, expectedPayer, inst.Payer().PublicKey.String())
	assert.Equal(t, expectedMint, inst.Mint().PublicKey.String())
	assert.Equal(t, expectedAuthority, inst.Authority().PublicKey.String())
	assert.Equal(t, expectedUserBank, inst.UserBank().PublicKey.String())
	assert.Equal(t, expectedEthAddress, inst.EthAddress.String())

	// Tests the EncodeToTree serialization
	buf := new(bytes.Buffer)
	encoder := text.NewTreeEncoder(buf, "")
	decoded.EncodeToTree(encoder)
	_, err = encoder.WriteString(encoder.Tree.String())
	require.NoError(t, err)
	s := buf.String()

	assert.Contains(t, s, expectedPayer)
	assert.Contains(t, s, expectedMint)
	assert.Contains(t, s, expectedAuthority)
	assert.Contains(t, s, expectedUserBank)
	assert.Contains(t, s, expectedEthAddress)
}

func TestBuildCreateInstruction(t *testing.T) {
	expectedProgramId := "Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ"
	expectedPayer := "9Thi2E1AfAqB3C7ZaUuLVV9QYM8UtPKumYwEBmcEUaB3"
	expectedMint := "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
	expectedAuthority := "5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx"
	expectedUserBank := "BsSCpqCamyBv3c9wyTFmZzmJmHtPpkw9ijC5whPQQFB"
	expectedEthAddress := "0x597daee17ce840bceed7e03f372057d74Ea7addA"
	expectedData, _ := hex.DecodeString("00597daee17ce840bceed7e03f372057d74ea7adda")

	// Test happy path
	{
		instBuilder, err := claimable_tokens.NewCreateTokenAccountInstruction(
			common.HexToAddress(expectedEthAddress),
			solana.MustPublicKeyFromBase58(expectedMint),
			solana.MustPublicKeyFromBase58(expectedPayer),
		)
		require.NoError(t, err)
		inst, err := instBuilder.ValidateAndBuild()
		require.NoError(t, err)

		assert.Equal(t, expectedProgramId, inst.ProgramID().String())

		accounts := inst.Accounts()
		assert.Equal(t, expectedPayer, accounts[0].PublicKey.String())
		assert.True(t, accounts[0].IsSigner)
		assert.True(t, accounts[0].IsWritable)
		assert.Equal(t, expectedMint, accounts[1].PublicKey.String())
		assert.False(t, accounts[1].IsSigner)
		assert.False(t, accounts[1].IsWritable)
		assert.Equal(t, expectedAuthority, accounts[2].PublicKey.String())
		assert.False(t, accounts[2].IsSigner)
		assert.False(t, accounts[2].IsWritable)
		assert.Equal(t, expectedUserBank, accounts[3].PublicKey.String())
		assert.False(t, accounts[3].IsSigner)
		assert.True(t, accounts[3].IsWritable)

		data, err := inst.Data()
		require.NoError(t, err)
		assert.Equal(t, expectedData, data)
	}

	// Test validation
	{
		dummyAddress := common.HexToAddress(expectedEthAddress)
		dummyPublicKey := solana.MustPublicKeyFromBase58(expectedPayer)
		// Missing ethAddress
		err := claimable_tokens.NewCreateTokenAccountInstructionBuilder().
			SetPayer(dummyPublicKey).
			SetMint(dummyPublicKey).
			SetAuthority(dummyPublicKey).
			SetUserBank(dummyPublicKey).
			Validate()
		assert.Error(t, err)

		// Missing payer
		err = claimable_tokens.NewCreateTokenAccountInstructionBuilder().
			SetEthAddress(dummyAddress).
			SetMint(dummyPublicKey).
			SetAuthority(dummyPublicKey).
			SetUserBank(dummyPublicKey).
			Validate()
		assert.Error(t, err)

		// Missing mint
		err = claimable_tokens.NewCreateTokenAccountInstructionBuilder().
			SetEthAddress(dummyAddress).
			SetPayer(dummyPublicKey).
			SetAuthority(dummyPublicKey).
			SetUserBank(dummyPublicKey).
			Validate()
		assert.Error(t, err)

		// Missing authority
		err = claimable_tokens.NewCreateTokenAccountInstructionBuilder().
			SetEthAddress(dummyAddress).
			SetPayer(dummyPublicKey).
			SetMint(dummyPublicKey).
			SetUserBank(dummyPublicKey).
			Validate()
		assert.Error(t, err)

		// Missing userBank
		_, err = claimable_tokens.NewCreateTokenAccountInstructionBuilder().
			SetEthAddress(dummyAddress).
			SetPayer(dummyPublicKey).
			SetMint(dummyPublicKey).
			SetAuthority(dummyPublicKey).
			ValidateAndBuild()
		assert.Error(t, err)

	}
}
