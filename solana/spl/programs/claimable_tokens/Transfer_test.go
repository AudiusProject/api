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

func TestDecodeTransferInstruction(t *testing.T) {
	// Real tx: 4cmqNs1Za4W6CmyeXHS8y7x1MQAqJYbXvjw7hCJwaC6aUBMbyhphRJohD4CEMpmqiwCowyeJcwKbghRAw39k4GB7
	tx, _ := solana.TransactionFromBase64("AbTmeFaIKVrCJAnM2bMCMYIjvujEcTjlDbBmuKo+RhhXjT3kQUGi86QM99qcWez9XWWrXZnJ/nZF4W/PWJYh6Q6AAQAIDKRMjXEaSfhdCSs3uSopl9yF1BCAPf5pyELNRmTsSDIkAjoKOvwZ2QM0MTsGEZg6zqP+rPfHMk+KHw39b3bQPcbhraxqhNsAOhA98jcD2tVl1UTjlz5nsnxh9MZJ2cXr0FzUHDQdAimd9k1conwhf506pvRBltFki+wbf9LW5pHdBMb8IPBQzPBVhNchHJ+M9Z7BR4W7FmoeKDDoEiAAAADPLvKviFfPDcCe1kbtn+YIorUsNzcMKO+I3ElSvGIHzUPP8mVBQWg25I3w6C+/HfRvUl5fLKmKECTCRggokeZlBqfVFxksXFEhjMlMPUrxf1ja7gibof1E49vZigAAAAAGp9UXGHvRZjXa1ARV/cLAwSTGjyFWdaXbustfCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABt324ddloZPZy+FGzut5rBy0he1fWzeROoz1hX7/AKkDBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAALIKB/PTujJbK2d4KZP330YqfykXUsNea4ZjLKkhoGafBAQAkQEBIAAADAAAYQAwAACv+b+ZN1X3wSPCNJ8IHoiqwFRKxBYPO7w4RhvOBJjKsYaMWN/FndW9xzmGVK2iNiI8GxYOMBOcOPh/pEnQXqNV4IR1HreaKE5QMHip2VvPHrS55SUB4a2saoTbADoQPfI3A9rVZdVE45c+Z7J8YfTGSdnF69AAViGDAAAAAAIAAAAAAAAABQkAAQIDBgcICQoVAa/5v5k3VffBI8I0nwgeiKrAVErECwAJA/BJAgAAAAAACwAFAoeOAAAA")

	expectedPayer := solana.MustPublicKeyFromBase58("C4MZpYiddDuWVofhs4BkUPyUiH78bFnaxhQVBB5fvko5")
	expectedSenderUserBank := solana.MustPublicKeyFromBase58("9h9UvjpGp2EGYKLGmbqnh5fkZV7t3dMQx3hncuTQUTX")
	expectedDestination := solana.MustPublicKeyFromBase58("GBxL8BQgATiLAvsQLNGFGjQvetTXZNDwwuBJX62efqqH")
	expectedNonce := solana.MustPublicKeyFromBase58("7FN6mjPX3KcwLWPuy8WW9stctXw9DgdKw8tkEoUtAq3r")
	expectedAuthority := solana.MustPublicKeyFromBase58("5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx")
	expectedSenderEthAddress := common.HexToAddress("01aff9bf993755f7c123c2349f081e88aac0544ac4")

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[1]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	decoded, err := claimable_tokens.DecodeInstruction(accounts, compiled.Data)
	require.NoError(t, err)
	inst, ok := decoded.Impl.(*claimable_tokens.Transfer)
	if !ok {
		assert.Fail(t, "bad type assert")
	}
	assert.Equal(t, expectedPayer, inst.Payer().PublicKey)
	assert.Equal(t, expectedSenderUserBank, inst.SenderUserBank().PublicKey)
	assert.Equal(t, expectedDestination, inst.Destination().PublicKey)
	assert.Equal(t, expectedNonce, inst.NonceAccount().PublicKey)
	assert.Equal(t, expectedAuthority, inst.Authority().PublicKey)
	assert.Equal(t, expectedSenderEthAddress, inst.SenderEthAddress)

	// Tests the EncodeToTree serialization
	buf := new(bytes.Buffer)
	encoder := text.NewTreeEncoder(buf, "")
	decoded.EncodeToTree(encoder)
	_, err = encoder.WriteString(encoder.Tree.String())
	require.NoError(t, err)
	s := buf.String()

	assert.Contains(t, s, expectedPayer.String())
	assert.Contains(t, s, expectedSenderUserBank.String())
	assert.Contains(t, s, expectedDestination.String())
	assert.Contains(t, s, expectedNonce.String())
	assert.Contains(t, s, expectedAuthority.String())
	assert.Contains(t, s, expectedSenderEthAddress.String())
}

func TestBuildTransferInstruction(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	expectedProgramId := solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ")
	expectedPayer := solana.MustPublicKeyFromBase58("C4MZpYiddDuWVofhs4BkUPyUiH78bFnaxhQVBB5fvko5")
	expectedSenderUserBank := solana.MustPublicKeyFromBase58("9h9UvjpGp2EGYKLGmbqnh5fkZV7t3dMQx3hncuTQUTX")
	expectedDestination := solana.MustPublicKeyFromBase58("GBxL8BQgATiLAvsQLNGFGjQvetTXZNDwwuBJX62efqqH")
	expectedNonce := solana.MustPublicKeyFromBase58("7FN6mjPX3KcwLWPuy8WW9stctXw9DgdKw8tkEoUtAq3r")
	expectedAuthority := solana.MustPublicKeyFromBase58("5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx")
	expectedSenderEthAddress := common.HexToAddress("01aff9bf993755f7c123c2349f081e88aac0544ac4")
	expectedData, _ := hex.DecodeString("01aff9bf993755f7c123c2349f081e88aac0544ac4")

	// Test happy path
	{
		instBuilder, err := claimable_tokens.NewTransferInstruction(
			expectedSenderEthAddress,
			mint,
			expectedPayer,
			expectedDestination,
		)
		require.NoError(t, err)
		inst, err := instBuilder.ValidateAndBuild()
		require.NoError(t, err)

		assert.Equal(t, expectedProgramId, inst.ProgramID())

		accounts := inst.Accounts()
		assert.Equal(t, expectedPayer, accounts[0].PublicKey)
		assert.True(t, accounts[0].IsSigner)
		assert.True(t, accounts[0].IsWritable)
		assert.Equal(t, expectedSenderUserBank, accounts[1].PublicKey)
		assert.False(t, accounts[1].IsSigner)
		assert.True(t, accounts[1].IsWritable)
		assert.Equal(t, expectedDestination, accounts[2].PublicKey)
		assert.False(t, accounts[2].IsSigner)
		assert.True(t, accounts[2].IsWritable)
		assert.Equal(t, expectedNonce, accounts[3].PublicKey)
		assert.False(t, accounts[3].IsSigner)
		assert.True(t, accounts[3].IsWritable)
		assert.Equal(t, expectedAuthority, accounts[4].PublicKey)
		assert.False(t, accounts[4].IsSigner)
		assert.False(t, accounts[4].IsWritable)
		assert.Equal(t, solana.SysVarRentPubkey, accounts[5].PublicKey)
		assert.False(t, accounts[5].IsSigner)
		assert.False(t, accounts[5].IsWritable)
		assert.Equal(t, solana.SysVarInstructionsPubkey, accounts[6].PublicKey)
		assert.False(t, accounts[6].IsSigner)
		assert.False(t, accounts[6].IsWritable)
		assert.Equal(t, solana.SystemProgramID, accounts[7].PublicKey)
		assert.False(t, accounts[7].IsSigner)
		assert.False(t, accounts[7].IsWritable)
		assert.Equal(t, solana.TokenProgramID, accounts[8].PublicKey)
		assert.False(t, accounts[8].IsSigner)
		assert.False(t, accounts[8].IsWritable)

		data, err := inst.Data()
		require.NoError(t, err)
		assert.Equal(t, expectedData, data)
	}
	// Test validation
	{
		// Missing senderEthAddress
		err := claimable_tokens.NewTransferInstructionBuilder().
			SetPayer(expectedPayer).
			SetSenderUserBank(expectedSenderUserBank).
			SetDestination(expectedDestination).
			SetNonceAccount(expectedNonce).
			SetAuthority(expectedAuthority).
			Validate()
		assert.Error(t, err)

		// Missing payer
		err = claimable_tokens.NewTransferInstructionBuilder().
			SetSenderEthAddress(expectedSenderEthAddress).
			SetSenderUserBank(expectedSenderUserBank).
			SetDestination(expectedDestination).
			SetNonceAccount(expectedNonce).
			SetAuthority(expectedAuthority).
			Validate()
		assert.Error(t, err)

		// Missing senderUserBank
		err = claimable_tokens.NewTransferInstructionBuilder().
			SetSenderEthAddress(expectedSenderEthAddress).
			SetPayer(expectedPayer).
			SetDestination(expectedDestination).
			SetNonceAccount(expectedNonce).
			SetAuthority(expectedAuthority).
			Validate()
		assert.Error(t, err)

		// Missing destination
		err = claimable_tokens.NewTransferInstructionBuilder().
			SetSenderEthAddress(expectedSenderEthAddress).
			SetPayer(expectedPayer).
			SetSenderUserBank(expectedSenderUserBank).
			SetNonceAccount(expectedNonce).
			SetAuthority(expectedAuthority).
			Validate()
		assert.Error(t, err)

		// Missing nonce
		err = claimable_tokens.NewTransferInstructionBuilder().
			SetSenderEthAddress(expectedSenderEthAddress).
			SetPayer(expectedPayer).
			SetSenderUserBank(expectedSenderUserBank).
			SetDestination(expectedDestination).
			SetAuthority(expectedAuthority).
			Validate()
		assert.Error(t, err)

		// Missing authority
		_, err = claimable_tokens.NewTransferInstructionBuilder().
			SetSenderEthAddress(expectedSenderEthAddress).
			SetPayer(expectedPayer).
			SetSenderUserBank(expectedSenderUserBank).
			SetDestination(expectedDestination).
			SetNonceAccount(expectedNonce).
			ValidateAndBuild()
		assert.Error(t, err)
	}
}
