package claimable_tokens_test

import (
	"testing"

	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
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
