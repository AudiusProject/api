package reward_manager_test

import (
	"encoding/base64"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	bin "github.com/gagliardetto/binary"
	"github.com/stretchr/testify/require"
)

func TestDecodeAttestationsAccount(t *testing.T) {
	testData, err := base64.StdEncoding.DecodeString("AeeCH+of/iC5SPD28pk1Fv5CZ/fBO1jttO39bISSUz9sBAC2Ri6VXaWEG22eHiUpuDDwDzG/nxMmaYsG6TJgMoFyC0Aizx+D7iJfAOH1BQAAAABfYjo1ZjI1NjEyOjI5MzVkYWNiAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAtkYulV2lhBttnh4lKbgw8A8xv/fJaRa9N6121O7dZTa4HClwbIBWnxMmaYsG6TJgMoFyC0Aizx+D7iJfAOH1BQAAAABfYjo1ZjI1NjEyOjI5MzVkYWNiXwC2Ri6VXaWEG22eHiUpuDDwDzG/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD3yWkWvTetdtTu3WU2uBwpcGyAVl6Yy+6qKs7ewIM6w9FjTip64PPCnxMmaYsG6TJgMoFyC0Aizx+D7iJfAOH1BQAAAABfYjo1ZjI1NjEyOjI5MzVkYWNiXwC2Ri6VXaWEG22eHiUpuDDwDzG/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABemMvuqirO3sCDOsPRY04qeuDzwo/PoQvTgIVwmH27Wx70q3RAD7/anxMmaYsG6TJgMoFyC0Aizx+D7iJfAOH1BQAAAABfYjo1ZjI1NjEyOjI5MzVkYWNiXwC2Ri6VXaWEG22eHiUpuDDwDzG/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACPz6EL04CFcJh9u1se9Kt0QA+/2gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==")
	require.NoError(t, err)

	result := reward_manager.AttestationsAccountData{}
	result.UnmarshalWithDecoder(bin.NewBinDecoder(testData))

	require.Equal(t, uint8(1), result.Version)
	require.Equal(t, "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq", result.RewardManagerState.String())
	require.Equal(t, uint8(4), result.Count)
	require.Len(t, result.Messages, 4)

	// Message 1
	require.Equal(t, "0x00b6462e955da5841b6d9e1e2529b830f00f31bf", result.Messages[0].SenderEthAddress)
	require.Equal(t, uint64(100000000), result.Messages[0].Claim.Amount)
	require.Equal(t, "b", result.Messages[0].Claim.RewardID)
	require.Equal(t, "5f25612:2935dacb", result.Messages[0].Claim.Specifier)
	require.Equal(t, "0x9f1326698b06e932603281720b4022cf1f83ee22", result.Messages[0].Claim.RecipientEthAddress)
	require.Equal(t, "", result.Messages[0].Claim.AntiAbuseOracleEthAddress)
	require.Equal(t, "0x00b6462e955da5841b6d9e1e2529b830f00f31bf", result.Messages[0].OperatorEthAddress)

	// Message 2
	require.Equal(t, "0xf7c96916bd37ad76d4eedd6536b81c29706c8056", result.Messages[1].SenderEthAddress)
	require.Equal(t, uint64(100000000), result.Messages[1].Claim.Amount)
	require.Equal(t, "b", result.Messages[1].Claim.RewardID)
	require.Equal(t, "5f25612:2935dacb", result.Messages[1].Claim.Specifier)
	require.Equal(t, "0x9f1326698b06e932603281720b4022cf1f83ee22", result.Messages[1].Claim.RecipientEthAddress)
	require.Equal(t, "0x00b6462e955da5841b6d9e1e2529b830f00f31bf", result.Messages[1].Claim.AntiAbuseOracleEthAddress)
	require.Equal(t, "0xf7c96916bd37ad76d4eedd6536b81c29706c8056", result.Messages[1].OperatorEthAddress)

	// Message 3
	require.Equal(t, "0x5e98cbeeaa2acedec0833ac3d1634e2a7ae0f3c2", result.Messages[2].SenderEthAddress)
	require.Equal(t, uint64(100000000), result.Messages[2].Claim.Amount)
	require.Equal(t, "b", result.Messages[2].Claim.RewardID)
	require.Equal(t, "5f25612:2935dacb", result.Messages[2].Claim.Specifier)
	require.Equal(t, "0x9f1326698b06e932603281720b4022cf1f83ee22", result.Messages[2].Claim.RecipientEthAddress)
	require.Equal(t, "0x00b6462e955da5841b6d9e1e2529b830f00f31bf", result.Messages[2].Claim.AntiAbuseOracleEthAddress)
	require.Equal(t, "0x5e98cbeeaa2acedec0833ac3d1634e2a7ae0f3c2", result.Messages[2].OperatorEthAddress)

	// Message 4
	require.Equal(t, "0x8fcfa10bd3808570987dbb5b1ef4ab74400fbfda", result.Messages[3].SenderEthAddress)
	require.Equal(t, uint64(100000000), result.Messages[3].Claim.Amount)
	require.Equal(t, "b", result.Messages[3].Claim.RewardID)
	require.Equal(t, "5f25612:2935dacb", result.Messages[3].Claim.Specifier)
	require.Equal(t, "0x9f1326698b06e932603281720b4022cf1f83ee22", result.Messages[3].Claim.RecipientEthAddress)
	require.Equal(t, "0x00b6462e955da5841b6d9e1e2529b830f00f31bf", result.Messages[3].Claim.AntiAbuseOracleEthAddress)
	require.Equal(t, "0x8fcfa10bd3808570987dbb5b1ef4ab74400fbfda", result.Messages[3].OperatorEthAddress)

}
