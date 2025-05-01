package secp256k1_test

import (
	"encoding/hex"
	"testing"

	"bridgerton.audius.co/api/spl/programs/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestSecp256k1Instruction(t *testing.T) {
	// Expected results
	ethAddress := common.HexToAddress("8fcfa10bd3808570987dbb5b1ef4ab74400fbfda")
	message, err := hex.DecodeString("68d5397bb16195ea47091010f3abb8fc6b5cdfa65f00e1f505000000005f623a33383639383d3e3530373431303135335f00b6462e955da5841b6d9e1e2529b830f00f31bf")
	require.NoError(t, err)
	signature, err := hex.DecodeString("f89b2e6f97f95f1306b468b10b1a18df9569b07d9d7b81b241d6fc99d9ec782e4e449f5c3c63836ed52c9344d3de5c3133fead711e421af545822f09bd78cb3900")
	require.NoError(t, err)

	// Test data (taken from Secp256k1Program.test.ts)
	expectedData, _ := hex.DecodeString("012000000c000061004500008fcfa10bd3808570987dbb5b1ef4ab74400fbfdaf89b2e6f97f95f1306b468b10b1a18df9569b07d9d7b81b241d6fc99d9ec782e4e449f5c3c63836ed52c9344d3de5c3133fead711e421af545822f09bd78cb390068d5397bb16195ea47091010f3abb8fc6b5cdfa65f00e1f505000000005f623a33383639383d3e3530373431303135335f00b6462e955da5841b6d9e1e2529b830f00f31bf")

	ix := secp256k1.NewSecp256k1Instruction(ethAddress, message, signature, 0).Build()
	data, err := ix.Data()

	require.Equal(t, solana.Secp256k1ProgramID, ix.ProgramID())
	require.Len(t, ix.Accounts(), 0)
	require.NoError(t, err)
	require.Equal(t, expectedData, data)
}

func TestUnmarshal(t *testing.T) {
	// Expected results
	ethAddress, err := hex.DecodeString("00b6462e955da5841b6d9e1e2529b830f00f31bf")
	require.NoError(t, err)
	message, err := hex.DecodeString("00ab2f814a75e9bb778ccbb998a028bb9b8a1ce1bc5f0065cd1d000000005f623a39636537613a3261633834376538")
	require.NoError(t, err)
	signature, err := hex.DecodeString("559ec22babe96e7d9ed0b40fd908a8da7049209cce80da05c46b4ed4b2ac996a16962d4d9b910f0bc536ee4bfe254a52cda3612f4e505ebacf8b0ea8869f6d4400")
	require.NoError(t, err)
	instrIndex := uint8(0)

	// Test data
	data, err := hex.DecodeString("012000000c000061002f000000b6462e955da5841b6d9e1e2529b830f00f31bf559ec22babe96e7d9ed0b40fd908a8da7049209cce80da05c46b4ed4b2ac996a16962d4d9b910f0bc536ee4bfe254a52cda3612f4e505ebacf8b0ea8869f6d440000ab2f814a75e9bb778ccbb998a028bb9b8a1ce1bc5f0065cd1d000000005f623a39636537613a3261633834376538")
	require.NoError(t, err)

	ix := secp256k1.NewSecp256k1InstructionBuilder()
	decoder := bin.NewBorshDecoder(data)
	ix.UnmarshalWithDecoder(decoder)

	require.Len(t, ix.SignatureDatas, 1)
	require.Equal(t, ethAddress, ix.SignatureDatas[0].EthAddress.Bytes())
	require.Equal(t, message, ix.SignatureDatas[0].Message)
	require.Equal(t, signature, ix.SignatureDatas[0].Signature)
	require.Equal(t, instrIndex, ix.SignatureDatas[0].InstructionIndex)
}

func TestUnmarshalVerifySignature(t *testing.T) {
	// Test data
	data, err := hex.DecodeString("012000000c0000610029000000b6462e955da5841b6d9e1e2529b830f00f31bf0b9e26079eabfde8da3ed9e3aa1d9a18d272bf50a11bc45ada709c9570b7b7825630f512be7673ff92cbdb5494b7e7890365d0eed04f46fc14b197dbf2f5529e0177afbe5f6e6d0b95e6b35ba205df8fbbf26f1d1f5f00c2eb0b000000005f66743a3162616561663731")
	require.NoError(t, err)

	ix := secp256k1.NewSecp256k1InstructionBuilder()
	decoder := bin.NewBorshDecoder(data)
	ix.UnmarshalWithDecoder(decoder)

	hash := crypto.Keccak256(ix.SignatureDatas[0].Message)
	recoveredWallet, err := crypto.SigToPub(hash, ix.SignatureDatas[0].Signature)

	require.NoError(t, err)
	require.Equal(
		t,
		ix.SignatureDatas[0].EthAddress.Bytes(),
		crypto.PubkeyToAddress(*recoveredWallet).Bytes(),
		"signature recovers to declared signer eth address",
	)
}
