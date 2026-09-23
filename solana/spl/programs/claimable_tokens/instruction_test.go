package claimable_tokens_test

import (
	"testing"

	"api.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func TestDecodeInstruction(t *testing.T) {
	// Real tx: 4tP4gk8Hj8GNat8AhVwHtjmDw9o9zM7qXr4uaT1wq4kV18LCDso1ZkUXsnbRsfJApYC1yNXf37JT57dgjfwGYWDm
	tx, _ := solana.TransactionFromBase64("AcJcCKM/J/nrKi2LUOYsnjw7Ax28UQgFrk+sk59pxdbh5hN6yPS/UCz7XMNOVheKvHE+Kv6pM84jGa2OTrdAzACAAQAIDCO+bWgH0qBU5b5AnZ7ziqP6XpBZi7QeH6AN7R+BzGSUEkw6SmwnmE36f7QiAJ7SnjyJv0h56Sa0ZYqCa+Ud2usnL2QsrKk7YBRWNGQTc7Oztsb59QgRjvJIaSH9ouW+tsaiIXcA38SjNQdOVqdLnj3XHd20cSPqsdyLq/TtqQA1BMb8IPBQzPBVhNchHJ+M9Z7BR4W7FmoeKDDoEiAAAAAb2qwxfmBGpjfrSx6WM8Ar6JnZcfv5KQxGH3OzE04GOpOTQjzbzruuwOo0PmzzsmbyUx8TNTtbvqvLYE7r9bNdBqfVFxksXFEhjMlMPUrxf1ja7gibof1E49vZigAAAAAGp9UXGHvRZjXa1ARV/cLAwSTGjyFWdaXbustfCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABt324ddloZPZy+FGzut5rBy0he1fWzeROoz1hX7/AKkDBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAALetMA5ipC4awp46PCZAeFJrlduBPU9BtcEJDQFwIyRQBAQAkQEBIAAADAAAYQAwAAD+9Um3cRwv0p0tJT+a2oqRB4OTPpe744t0zo1w8ut38TiYezeMB3ahQO6/rFO00hy9LY7qFTDF+mNrWd5LSxV5h/hkRVBnUC6kU1nKGS9bxftmzsEAJy9kLKypO2AUVjRkE3Ozs7bG+fUIEY7ySGkh/aLlvrYAZc0dAAAAACsAAAAAAAAABQkAAQIDBgcICQoVAf71SbdxHC/SnS0lP5raipEHg5M+CwAJA/BJAgAAAAAACwAFAqS5AAAA")

	originalProgramId := solana.MustPublicKeyFromBase58("Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ")
	expectedProgramId := solana.MustPublicKeyFromBase58("2sjQNmUfkV6yKKi4dPR8gWRgtyma5aiymE3aXL2RAZww")

	// Test decoding from the transaction
	compiled := tx.Message.Instructions[0]
	accounts, err := compiled.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	before, err := solana.DecodeInstruction(originalProgramId, accounts, compiled.Data)
	require.NoError(t, err)
	claimable_tokens.SetProgramID(expectedProgramId)
	after, err := solana.DecodeInstruction(expectedProgramId, accounts, compiled.Data)
	require.NoError(t, err)

	assert.Equal(t, before, after)

	claimable_tokens.SetProgramID(originalProgramId)

}

func TestDecodeSetAuthorityInstruction(t *testing.T) {
	// Real tx: 2RuRg4MCAJHF5dQifhGzkZ6qD7XQGbGop4Z48fxgweHawi5FNKiwiTVYwfQ7hzgU1MgJkjaKV7SWumr8ztGa98cf
	accounts := []*solana.AccountMeta{
		solana.Meta(solana.MustPublicKeyFromBase58("8teuqGNB7RhwC2JoDT961J3jStBfXeu4nRxyubu36Kpe")).WRITE(),
		solana.Meta(solana.MustPublicKeyFromBase58("5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx")),
		solana.Meta(solana.SysVarInstructionsPubkey),
		solana.Meta(solana.SysVarRecentBlockHashesPubkey),
		solana.Meta(solana.TokenProgramID),
	}

	decoded, err := claimable_tokens.DecodeInstruction(
		accounts,
		[]byte{claimable_tokens.Instruction_SetAuthority},
	)
	require.NoError(t, err)
	require.IsType(t, &claimable_tokens.SetAuthority{}, decoded.Impl)
	assert.Equal(t, accounts, decoded.Accounts())
}

func TestDecodeCloseInstruction(t *testing.T) {
	ethAddress := common.HexToAddress("0x65b2681094e762CAD2349e6C1e2FbC8d54b9c046")
	accounts := []*solana.AccountMeta{
		solana.Meta(solana.NewWallet().PublicKey()).WRITE(),
		solana.Meta(solana.NewWallet().PublicKey()),
		solana.Meta(solana.NewWallet().PublicKey()).WRITE(),
		solana.Meta(solana.TokenProgramID),
	}
	data := append([]byte{claimable_tokens.Instruction_Close}, ethAddress.Bytes()...)

	decoded, err := claimable_tokens.DecodeInstruction(accounts, data)
	require.NoError(t, err)
	closeInstruction, ok := decoded.Impl.(*claimable_tokens.Close)
	require.True(t, ok)
	assert.Equal(t, ethAddress, closeInstruction.EthAddress)
	assert.Equal(t, accounts, decoded.Accounts())
}
