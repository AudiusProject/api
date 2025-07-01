package payment_router_test

import (
	"encoding/hex"
	"testing"

	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestDecodeInstruction(t *testing.T) {
	// Real transaction: 5bLATdRvuJWcdBWa5RzDZ88DZYMDMkwHW7ER6oQcifrTHSQP4iRdfJKCmnmsvzbuEDUrWwg665vWXKGSn9Gec6p6
	tx, _ := solana.TransactionFromBase64("AeWsWIS+LHWST5LaEz9OZFfrjh0l6/x3P0xuJgXW62T28mrapo2IKMkaPwlIsV7wpGIKHUe/1sfmzEqJhIsFWQeAAQALEfWhZqfxE5Xt0EE0UwVkk8pyjm+FLYRDrQyD0ojyLO86FfijnIT0NRNgcQQcoOHfqxAF+jfC0CZJW0h/zbH2Z8BhM0Gn3spQ9Fee8x2/dhn6ij/lg+GJJk70ksJoWr0WPs/I0PdJTTie2H7yiLehL1kWC8H+giztxunPaHeK8q2CxKgcMcZFB6pKan4hjgAWtz9aj8wPSmcKLv06EBe8PDZmywqVd7SHXkdj4ki3Nffd0GmHGvL4gB4zHYCiU+DAOgTG/CDwUMzwVYTXIRyfjPWewUeFuxZqHigw6BIgAAAAzy7yr4hXzw3AntZG7Z/mCKK1LDc3DCjviNxJUrxiB83QQBm+2natZtdlxJC/tn3eX37kUUln29E+oByMpxOuDQan1RcZLFxRIYzJTD1K8X9Y2u4Im6H9ROPb2YoAAAAABqfVFxh70WY12tQEVf3CwMEkxo8hVnWl27rLXwgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAbd9uHXZaGT2cvhRs7reawctIXtX1s3kTqM9YV+/wCpDDC4ZKXZ8xNtdKlw5YBMugbzGaZaVUM7bXvwET7Z401s4UD/vcjS+CXoLDxmXaspL5SyJbaD7kBrJ/jiuJwQvgVKU1qZKSEGTSTocWDaOHx8NbXdvJK7geQfqEBBBUSNAwZGb+UhFzL/7K26csOb57yM5bvF9xJrLEObOkAAAAALnOll0PQwz8TD5YAcAxrbZJ6y/ktzAx3qd7oswJoydwcGAJEBASAAAAwAAGEAMAAAGDDpLA4atNHv8/KWwpMuvq3CbOOZq3BAcY4XeRfr6mJ1omyOJCCGcOE1yxhaRia9yyhJuhm6orVqbu7MNsO+lXTd5i74Vz1+h3D7XLRkGk05jr1eAWEzQafeylD0V57zHb92GfqKP+WD4YkmTvSSwmhavRY+kMkZAAAAAAAfAAAAAAAAAAcJAAECAwgJCgsMFQEYMOksDhq00e/z8pbCky6+rcJs4w0FAg4MBAUl5RfLl3rjrSr+AgAAAGg1FwAAAAAAKJQCAAAAAACQyRkAAAAAAA8AKXRyYWNrOjIxMDc4OTIyOTc6NzM1NDkxMjY6MTI4MDA2OmRvd25sb2FkDwBFZ2VvOnsiY2l0eSI6Ik5ld2FyayIsInJlZ2lvbiI6Ik5ldyBKZXJzZXkiLCJjb3VudHJ5IjoiVW5pdGVkIFN0YXRlcyJ9EAAJA/BJAgAAAAAAEAAFAgcQAgAA")
	expectedSender := solana.MustPublicKeyFromBase58("7YRsw96JjbLKfXY51c64kSvTK8opgxw292GT8J1HGKf3")
	expectedSenderOwner := solana.MustPublicKeyFromBase58("8L2FL5g9y9CzAFY1471tLAXBUsupdp1kNeFuP648mqxR")
	expectedDest1 := solana.MustPublicKeyFromBase58("EEfb12rCT8tCKB9wpHq5183x59g8Vyex6pPwq2uLQLcM")
	expectedDest2 := solana.MustPublicKeyFromBase58("7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf")
	expectedAmount1 := uint64(1521000)
	expectedAmount2 := uint64(169000)
	expectedTotal := uint64(1690000)

	compiledRouteInst := tx.Message.Instructions[2]
	accounts, err := compiledRouteInst.ResolveInstructionAccounts(&tx.Message)
	require.NoError(t, err)
	decoded, err := payment_router.DecodeInstruction(accounts, compiledRouteInst.Data)
	require.NoError(t, err)
	routeInst, ok := decoded.Impl.(*payment_router.Route)
	if !ok {
		assert.Fail(t, "bad type assert")
	}
	assert.Equal(t, expectedSender, routeInst.GetSender().PublicKey)
	assert.Equal(t, expectedSenderOwner, routeInst.GetSenderOwner().PublicKey)
	assert.Equal(t, expectedDest1, routeInst.GetDestinations()[0].PublicKey)
	assert.Equal(t, expectedDest2, routeInst.GetDestinations()[1].PublicKey)
	assert.Equal(t, expectedAmount1, routeInst.Amounts[0])
	assert.Equal(t, expectedAmount2, routeInst.Amounts[1])
	assert.Equal(t, expectedTotal, routeInst.TotalAmount)
}

func TestNewRouteInstruction(t *testing.T) {
	expectedSender := solana.MustPublicKeyFromBase58("7YRsw96JjbLKfXY51c64kSvTK8opgxw292GT8J1HGKf3")
	expectedSenderOwner := solana.MustPublicKeyFromBase58("8L2FL5g9y9CzAFY1471tLAXBUsupdp1kNeFuP648mqxR")
	expectedDest1 := solana.MustPublicKeyFromBase58("EEfb12rCT8tCKB9wpHq5183x59g8Vyex6pPwq2uLQLcM")
	expectedDest2 := solana.MustPublicKeyFromBase58("7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf")
	expectedAmount1 := uint64(1521000)
	expectedAmount2 := uint64(169000)
	expectedPaymentRouterPdaBump := uint8(254)
	expectedData, _ := hex.DecodeString("e517cb977ae3ad2afe020000006835170000000000289402000000000090c9190000000000")

	inst := payment_router.NewRouteInstruction(
		expectedSender,
		expectedSenderOwner,
		expectedPaymentRouterPdaBump,
		map[solana.PublicKey]uint64{
			expectedDest1: expectedAmount1,
			expectedDest2: expectedAmount2,
		},
	).Build()

	accounts := inst.Accounts()
	assert.Equal(t, expectedSender, accounts[0].PublicKey)
	assert.True(t, accounts[0].IsWritable)
	assert.Equal(t, expectedSenderOwner, accounts[1].PublicKey)
	assert.False(t, accounts[1].IsWritable)
	assert.Equal(t, solana.TokenProgramID, accounts[2].PublicKey)
	assert.False(t, accounts[2].IsWritable)
	assert.Equal(t, expectedDest1, accounts[3].PublicKey)
	assert.True(t, accounts[3].IsWritable)
	assert.Equal(t, expectedDest2, accounts[4].PublicKey)
	assert.True(t, accounts[4].IsWritable)

	data, err := inst.Data()
	require.NoError(t, err)
	assert.Equal(t, expectedData, data)
}

func TestRouteMap(t *testing.T) {
	expectedDest1 := solana.MustPublicKeyFromBase58("EEfb12rCT8tCKB9wpHq5183x59g8Vyex6pPwq2uLQLcM")
	expectedDest2 := solana.MustPublicKeyFromBase58("7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf")

	expectedRouteMap1 := map[solana.PublicKey]uint64{
		expectedDest1: uint64(1),
		expectedDest2: uint64(2),
	}
	expectedRouteMap2 := map[solana.PublicKey]uint64{
		expectedDest1: uint64(3),
		expectedDest2: uint64(4),
	}

	inst := payment_router.NewRouteInstructionBuilder().
		SetRouteMap(expectedRouteMap1)
	assert.Equal(t, expectedRouteMap1, inst.GetRouteMap())

	inst.SetRouteMap(expectedRouteMap2)
	assert.Equal(t, expectedRouteMap2, inst.GetRouteMap())
}

func TestRouteEncodeToTree(t *testing.T) {
	// Real transaction: 5bLATdRvuJWcdBWa5RzDZ88DZYMDMkwHW7ER6oQcifrTHSQP4iRdfJKCmnmsvzbuEDUrWwg665vWXKGSn9Gec6p6
	tx, _ := solana.TransactionFromBase64("AeWsWIS+LHWST5LaEz9OZFfrjh0l6/x3P0xuJgXW62T28mrapo2IKMkaPwlIsV7wpGIKHUe/1sfmzEqJhIsFWQeAAQALEfWhZqfxE5Xt0EE0UwVkk8pyjm+FLYRDrQyD0ojyLO86FfijnIT0NRNgcQQcoOHfqxAF+jfC0CZJW0h/zbH2Z8BhM0Gn3spQ9Fee8x2/dhn6ij/lg+GJJk70ksJoWr0WPs/I0PdJTTie2H7yiLehL1kWC8H+giztxunPaHeK8q2CxKgcMcZFB6pKan4hjgAWtz9aj8wPSmcKLv06EBe8PDZmywqVd7SHXkdj4ki3Nffd0GmHGvL4gB4zHYCiU+DAOgTG/CDwUMzwVYTXIRyfjPWewUeFuxZqHigw6BIgAAAAzy7yr4hXzw3AntZG7Z/mCKK1LDc3DCjviNxJUrxiB83QQBm+2natZtdlxJC/tn3eX37kUUln29E+oByMpxOuDQan1RcZLFxRIYzJTD1K8X9Y2u4Im6H9ROPb2YoAAAAABqfVFxh70WY12tQEVf3CwMEkxo8hVnWl27rLXwgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAbd9uHXZaGT2cvhRs7reawctIXtX1s3kTqM9YV+/wCpDDC4ZKXZ8xNtdKlw5YBMugbzGaZaVUM7bXvwET7Z401s4UD/vcjS+CXoLDxmXaspL5SyJbaD7kBrJ/jiuJwQvgVKU1qZKSEGTSTocWDaOHx8NbXdvJK7geQfqEBBBUSNAwZGb+UhFzL/7K26csOb57yM5bvF9xJrLEObOkAAAAALnOll0PQwz8TD5YAcAxrbZJ6y/ktzAx3qd7oswJoydwcGAJEBASAAAAwAAGEAMAAAGDDpLA4atNHv8/KWwpMuvq3CbOOZq3BAcY4XeRfr6mJ1omyOJCCGcOE1yxhaRia9yyhJuhm6orVqbu7MNsO+lXTd5i74Vz1+h3D7XLRkGk05jr1eAWEzQafeylD0V57zHb92GfqKP+WD4YkmTvSSwmhavRY+kMkZAAAAAAAfAAAAAAAAAAcJAAECAwgJCgsMFQEYMOksDhq00e/z8pbCky6+rcJs4w0FAg4MBAUl5RfLl3rjrSr+AgAAAGg1FwAAAAAAKJQCAAAAAACQyRkAAAAAAA8AKXRyYWNrOjIxMDc4OTIyOTc6NzM1NDkxMjY6MTI4MDA2OmRvd25sb2FkDwBFZ2VvOnsiY2l0eSI6Ik5ld2FyayIsInJlZ2lvbiI6Ik5ldyBKZXJzZXkiLCJjb3VudHJ5IjoiVW5pdGVkIFN0YXRlcyJ9EAAJA/BJAgAAAAAAEAAFAgcQAgAA")

	expectedSender := solana.MustPublicKeyFromBase58("7YRsw96JjbLKfXY51c64kSvTK8opgxw292GT8J1HGKf3")
	expectedSenderOwner := solana.MustPublicKeyFromBase58("8L2FL5g9y9CzAFY1471tLAXBUsupdp1kNeFuP648mqxR")
	expectedDest1 := solana.MustPublicKeyFromBase58("EEfb12rCT8tCKB9wpHq5183x59g8Vyex6pPwq2uLQLcM")
	expectedDest2 := solana.MustPublicKeyFromBase58("7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf")
	expectedAmount1 := "1521000"
	expectedAmount2 := "169000"

	s := tx.String()

	assert.Contains(t, s, expectedSender.String())
	assert.Contains(t, s, expectedSenderOwner.String())
	assert.Contains(t, s, expectedDest1.String())
	assert.Contains(t, s, expectedDest2.String())
	assert.Contains(t, s, expectedAmount1)
	assert.Contains(t, s, expectedAmount2)
}
