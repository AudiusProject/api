package claimable_tokens

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestDeriveUserBankAccount(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	ethAddress := common.HexToAddress("0xa507da823bf0c5dc44a759d0d398b7f52097da19")
	expectedUserBankAccount := solana.MustPublicKeyFromBase58("9oJLynXRLkWZkTXXExPXVbza5n8CzTZLvtJ1Y3pEJ2Pk")

	userBankAccount, err := deriveUserBankAccount(mint, ethAddress)
	require.NoError(t, err)
	require.Equal(t, expectedUserBankAccount.String(), userBankAccount.String())
}
