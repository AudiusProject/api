package claimable_tokens_test

import (
	"testing"

	"bridgerton.audius.co/api/spl/programs/claimable_tokens"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestDeriveUserBankAccount(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	ethAddress := "0xa507da823bf0c5dc44a759d0d398b7f52097da19"
	expectedUserBankAccount := solana.MustPublicKeyFromBase58("9oJLynXRLkWZkTXXExPXVbza5n8CzTZLvtJ1Y3pEJ2Pk")

	userBankAccount, err := claimable_tokens.DeriveUserBankAccount(mint, ethAddress)
	require.NoError(t, err)
	require.Equal(t, expectedUserBankAccount.String(), userBankAccount.String())
}
