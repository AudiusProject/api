package reward_manager_test

import (
	"encoding/hex"
	"testing"

	"bridgerton.audius.co/api/spl/programs/reward_manager"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestEvaluateAttestationsInstruction(t *testing.T) {
	// Test data
	challengeId := "ft"
	specifier := "37364e80"
	recipientEthAddress := common.HexToAddress("0x3f6d9fcf0d4466dd5886e3b1def017adfb7916b4")
	amount := uint64(200000000)
	antiAbuseOracleEthAddress := common.HexToAddress("0x00b6462e955dA5841b6D9e1E2529B830F00f31Bf")

	// Expected Accounts
	// From successful stage transaction (signature 26gT9HVMhzBDzsKcsiKREYmGcXuZhjAJpCVUu9WFNhVMyKje8SdApYc4ev3HrumZB4LEXLUaPnKyriBPLmtzwrWp)
	rewardState := solana.MustPublicKeyFromBase58("GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq")
	expectedAuthority := solana.MustPublicKeyFromBase58("6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa")
	tokenSource := solana.MustPublicKeyFromBase58("HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ")
	destinationUserBank := solana.MustPublicKeyFromBase58("Cjv8dvVfWU8wUYAR82T5oZ4nHLB6EyGNvpPBzw3r76Qy")
	expectedDisbursement := solana.MustPublicKeyFromBase58("3qQfuDEBWEmxRo5G4J2a4eYUVf9u1LWzLgRPndiwew2w")
	expectedOracle := solana.MustPublicKeyFromBase58("FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk")
	payer := solana.MustPublicKeyFromBase58("E3CfijtAJwBSHfwFEViAUd3xp7c8TBxwC1eXn1Fgxp8h")

	// Expected Data (from same tx)
	expectedData, err := hex.DecodeString("0700c2eb0b000000000b00000066743a33373336346538303f6d9fcf0d4466dd5886e3b1def017adfb7916b4")
	require.NoError(t, err)

	// Use stage program ID
	stageProgramId := solana.MustPublicKeyFromBase58("CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp")
	reward_manager.SetProgramID(stageProgramId)

	inst := reward_manager.NewEvaluateAttestationInstruction(
		challengeId,
		specifier,
		recipientEthAddress,
		amount,
		antiAbuseOracleEthAddress,
		rewardState,
		tokenSource,
		destinationUserBank,
		payer,
	).Build()

	require.Equal(t, stageProgramId, inst.ProgramID())
	require.Len(t, inst.Accounts(), 11)
	require.Equal(t, rewardState.String(), inst.Accounts()[1].PublicKey.String())
	require.Equal(t, expectedAuthority.String(), inst.Accounts()[2].PublicKey.String())
	require.Equal(t, tokenSource.String(), inst.Accounts()[3].PublicKey.String())
	require.Equal(t, destinationUserBank.String(), inst.Accounts()[4].PublicKey.String())
	require.Equal(t, expectedDisbursement.String(), inst.Accounts()[5].PublicKey.String())
	require.Equal(t, expectedOracle.String(), inst.Accounts()[6].PublicKey.String())
	require.Equal(t, payer.String(), inst.Accounts()[7].PublicKey.String())

	data, err := inst.Data()
	require.NoError(t, err)
	require.Equal(t, expectedData, data)
}
