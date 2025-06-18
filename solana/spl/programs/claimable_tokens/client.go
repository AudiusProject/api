package claimable_tokens

import (
	"context"

	"bridgerton.audius.co/solana/spl"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type ClaimableTokensClient struct {
	client *rpc.Client
	sender *spl.TransactionSender
}

func NewClaimableTokensClient(
	client *rpc.Client,
	programId solana.PublicKey,
	transactionSender *spl.TransactionSender,
) (*ClaimableTokensClient, error) {
	return &ClaimableTokensClient{
		client: client,
		sender: transactionSender,
	}, nil
}

func (cc *ClaimableTokensClient) CreateUserBank(
	ctx context.Context,
	ethAddress common.Address,
	mint solana.PublicKey,
) error {
	payer := cc.sender.GetFeePayer()
	inst, err := NewCreateTokenAccountInstruction(ethAddress, mint, payer.PublicKey()).
		ValidateAndBuild()
	if err != nil {
		return err
	}

	tx := solana.NewTransactionBuilder().
		SetFeePayer(payer.PublicKey()).
		AddInstruction(inst)

	cc.sender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{
		Percentile: 99,
		Multiplier: 1,
	})
	cc.sender.AddComputeBudgetLimit(ctx, tx, spl.AddComputeBudgetLimitParams{
		Padding:    1000,
		Multiplier: 1.2,
	})
	_, err = cc.sender.SendTransactionWithRetries(ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
	if err != nil {
		return err
	}
	return nil
}

func (cc *ClaimableTokensClient) GetOrCreateUserBank(
	ctx context.Context,
	ethAddress common.Address,
	mint solana.PublicKey,
) (*solana.PublicKey, error) {
	userBank, err := deriveUserBankAccount(mint, ethAddress)
	if err != nil {
		return nil, err
	}

	_, err = cc.client.GetAccountInfo(ctx, userBank)
	if err != nil {
		if err.Error() == "not found" {
			err = cc.CreateUserBank(ctx, ethAddress, mint)
			if err != nil {
				return nil, err
			}
		}
	}

	return &userBank, nil
}
