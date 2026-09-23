package jobs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"api.audius.co/config"
	"api.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeReclaimRentRPC struct {
	result *rpc.GetMultipleAccountsResult
	err    error
}

func (f fakeReclaimRentRPC) GetMultipleAccountsWithOpts(
	_ context.Context,
	_ []solana.PublicKey,
	_ *rpc.GetMultipleAccountsOpts,
) (*rpc.GetMultipleAccountsResult, error) {
	return f.result, f.err
}

type fakeReclaimRentSender struct {
	wallet *solana.Wallet
	tx     *solana.Transaction
	err    error
}

func (f *fakeReclaimRentSender) GetFeePayer() (*solana.Wallet, error) {
	return f.wallet, nil
}

func (f *fakeReclaimRentSender) SendTransactionWithRetries(
	_ context.Context,
	builder *solana.TransactionBuilder,
	_ rpc.CommitmentType,
	_ rpc.TransactionOpts,
) (*solana.Signature, error) {
	if f.err != nil {
		return nil, f.err
	}
	tx, err := builder.Build()
	if err != nil {
		return nil, err
	}
	f.tx = tx
	sig := solana.Signature{1}
	return &sig, nil
}

func encodeTokenAccount(t *testing.T, account token.Account) *rpc.DataBytesOrJSON {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, bin.NewBinEncoder(&buf).Encode(account))
	return rpc.DataBytesOrJSONFromBytes(buf.Bytes())
}

func TestReclaimRentFilterOnChain(t *testing.T) {
	mint := solana.NewWallet().PublicKey()
	otherMint := solana.NewWallet().PublicKey()
	authority := solana.NewWallet().PublicKey()
	otherAuthority := solana.NewWallet().PublicKey()

	makeInfo := func(programOwner solana.PublicKey, account token.Account) *rpc.Account {
		return &rpc.Account{Owner: programOwner, Data: encodeTokenAccount(t, account)}
	}
	baseAccount := token.Account{
		Mint:   mint,
		Owner:  authority,
		State:  token.Initialized,
		Amount: 0,
	}
	wrongMintAccount := baseAccount
	wrongMintAccount.Mint = otherMint
	nonZeroAccount := baseAccount
	nonZeroAccount.Amount = 1
	wrongOwnerAccount := baseAccount
	wrongOwnerAccount.Owner = otherAuthority
	closeAuthorityAccount := wrongOwnerAccount
	closeAuthorityAccount.CloseAuthority = &authority
	externalCloseAuthorityAccount := baseAccount
	externalCloseAuthorityAccount.CloseAuthority = &otherAuthority

	accounts := make([]reclaimRentAccount, 11)
	for i := range accounts {
		accounts[i].EthereumAddress = fmt.Sprintf("0x%040x", i+1)
		userBank, err := claimable_tokens.DeriveUserBankAccount(
			mint,
			common.HexToAddress(accounts[i].EthereumAddress),
		)
		require.NoError(t, err)
		accounts[i].Account = userBank.String()
	}
	accounts[9].EthereumAddress = "not-an-ethereum-address"
	accounts[10].Account = solana.NewWallet().PublicKey().String()

	job := &ReclaimRentJob{
		rpcClient: fakeReclaimRentRPC{result: &rpc.GetMultipleAccountsResult{Value: []*rpc.Account{
			nil,
			makeInfo(solana.SystemProgramID, baseAccount),
			{Owner: solana.TokenProgramID, Data: rpc.DataBytesOrJSONFromBytes([]byte{1, 2, 3})},
			makeInfo(solana.TokenProgramID, wrongMintAccount),
			makeInfo(solana.TokenProgramID, nonZeroAccount),
			makeInfo(solana.TokenProgramID, wrongOwnerAccount),
			makeInfo(solana.TokenProgramID, baseAccount),
			makeInfo(solana.TokenProgramID, closeAuthorityAccount),
			makeInfo(solana.TokenProgramID, externalCloseAuthorityAccount),
			makeInfo(solana.TokenProgramID, baseAccount),
			makeInfo(solana.TokenProgramID, baseAccount),
		}}},
		logger: zap.NewNop(),
	}

	filtered, stats, err := job.filterOnChain(context.Background(), accounts, mint, authority)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
	assert.Equal(t, accounts[6], filtered[0])
	assert.Equal(t, accounts[7], filtered[1])
	assert.Equal(t, reclaimRentFilterStats{
		MissingAccount:      1,
		InvalidOwner:        1,
		InvalidData:         1,
		MintMismatch:        1,
		NonZeroBalance:      1,
		InvalidEthAddress:   1,
		AddressMismatch:     1,
		WrongCloseAuthority: 2,
	}, stats)
}

func TestReclaimRentProcessBatchUsesProgramDestination(t *testing.T) {
	sender := &fakeReclaimRentSender{wallet: solana.NewWallet()}
	job := &ReclaimRentJob{transactionSender: sender}
	authority := solana.NewWallet().PublicKey()
	account := reclaimRentAccount{
		Account:         solana.NewWallet().PublicKey().String(),
		EthereumAddress: "0x1234567890123456789012345678901234567890",
	}

	_, err := job.processBatch(context.Background(), []reclaimRentAccount{account}, authority)
	require.NoError(t, err)
	require.NotNil(t, sender.tx)
	require.Len(t, sender.tx.Message.Instructions, 1)

	metas, err := sender.tx.Message.Instructions[0].ResolveInstructionAccounts(&sender.tx.Message)
	require.NoError(t, err)
	require.Len(t, metas, 4)
	assert.Equal(t, claimable_tokens.DefaultRentDestinationAddress, metas[2].PublicKey.String())
	assert.NotEqual(t, sender.wallet.PublicKey(), metas[2].PublicKey)
}

func TestReclaimRentProcessBatchReturnsSenderError(t *testing.T) {
	expectedErr := errors.New("send failed")
	job := &ReclaimRentJob{transactionSender: &fakeReclaimRentSender{
		wallet: solana.NewWallet(),
		err:    expectedErr,
	}}

	_, err := job.processBatch(context.Background(), []reclaimRentAccount{{
		Account:         solana.NewWallet().PublicKey().String(),
		EthereumAddress: "0x1234567890123456789012345678901234567890",
	}}, solana.NewWallet().PublicKey())
	assert.ErrorIs(t, err, expectedErr)
}

func TestReclaimRentRunReturnsBatchError(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer pool.Close()

	mint := solana.NewWallet().PublicKey()
	authority, _, err := claimable_tokens.DeriveAuthority(mint)
	require.NoError(t, err)
	ethAddress := "0x1234567890123456789012345678901234567890"
	account, err := claimable_tokens.DeriveUserBankAccount(mint, common.HexToAddress(ethAddress))
	require.NoError(t, err)

	pool.ExpectQuery("SELECT DISTINCT").
		WithArgs(mint.String(), pgxmock.AnyArg(), reclaimRentDbPageSize, 0).
		WillReturnRows(pgxmock.NewRows([]string{"account", "ethereum_address"}).
			AddRow(account.String(), ethAddress))
	pool.ExpectQuery("SELECT DISTINCT").
		WithArgs(mint.String(), pgxmock.AnyArg(), reclaimRentDbPageSize, 1).
		WillReturnRows(pgxmock.NewRows([]string{"account", "ethereum_address"}))

	expectedErr := errors.New("send failed")
	wallet := solana.NewWallet()
	job := &ReclaimRentJob{
		cfg: config.Config{SolanaConfig: config.SolanaConfig{
			FeePayers: []solana.Wallet{*wallet},
		}},
		pool: pool,
		rpcClient: fakeReclaimRentRPC{result: &rpc.GetMultipleAccountsResult{Value: []*rpc.Account{{
			Owner: solana.TokenProgramID,
			Data: encodeTokenAccount(t, token.Account{
				Mint:  mint,
				Owner: authority,
				State: token.Initialized,
			}),
		}}}},
		transactionSender: &fakeReclaimRentSender{wallet: wallet, err: expectedErr},
		mints:             []solana.PublicKey{mint},
		logger:            zap.NewNop(),
	}

	err = job.run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "1 processing operation(s) failed")
	require.NoError(t, pool.ExpectationsWereMet())
}
