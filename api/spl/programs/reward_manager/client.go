package reward_manager

import (
	"context"

	"bridgerton.audius.co/api/spl"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

// A client for the Reward Manager Program that's immutably preconfigured to an
// instance of the program.
type RewardManagerClient struct {
	client                    *rpc.Client
	rewardManagerStateAccount solana.PublicKey
	authority                 solana.PublicKey
	rewardManagerState        *RewardManagerState
	lookupTableAccount        solana.PublicKey
	lookupTable               *spl.AddressLookupTable
	logger                    *zap.Logger
}

// Creates a RewardManagerClient for the instance defined by the programId and
// state account.
func NewRewardManagerClient(
	client *rpc.Client,
	programId solana.PublicKey,
	stateAccount solana.PublicKey,
	lookupTable solana.PublicKey,
	logger *zap.Logger,
) (*RewardManagerClient, error) {
	authority, _, err := deriveAuthorityAccount(programId, stateAccount)
	if err != nil {
		return nil, err
	}
	return &RewardManagerClient{
		client:                    client,
		rewardManagerStateAccount: stateAccount,
		authority:                 authority,
		lookupTableAccount:        lookupTable,
		rewardManagerState:        nil,
		logger:                    logger.Named("RewardManagerClient"),
	}, nil
}

// Gets the data of the rewardManagerState account, which has the configuration
// for this instance of the RewardManagerProgram on chain.
func (rc *RewardManagerClient) GetProgramState(ctx context.Context) (*RewardManagerState, error) {
	if rc.rewardManagerState != nil {
		return rc.rewardManagerState, nil
	}

	err := rc.client.GetAccountDataBorshInto(ctx, rc.rewardManagerStateAccount, &rc.rewardManagerState)
	if err != nil {
		return nil, err
	}
	return rc.rewardManagerState, nil
}

// Gets the public key of the rewardManagerState account for this instance.
func (rc *RewardManagerClient) GetProgramStateAccount() solana.PublicKey {
	return rc.rewardManagerStateAccount
}

// Gets the lookup table that has all the registered senders and other accounts
// used in most RewardManagerProgram instructions.
func (rc *RewardManagerClient) GetLookupTable(ctx context.Context) (*spl.AddressLookupTable, error) {
	if rc.lookupTable != nil {
		return rc.lookupTable, nil
	}

	err := rc.client.GetAccountDataInto(ctx, rc.lookupTableAccount, &rc.lookupTable)
	if err != nil {
		return nil, err
	}
	return rc.lookupTable, nil
}

// Gets the public key of the lookup table that has all the registered senders
// and other accounts used in most RewardManagerProgram instructions.
func (rc *RewardManagerClient) GetLookupTableAccount() solana.PublicKey {
	return rc.lookupTableAccount
}

// Gets the claims already submitted for a rewards claim from the account data.
func (rc *RewardManagerClient) GetSubmittedAttestations(
	ctx context.Context,
	claim rewards.RewardClaim,
) (*AttestationsAccountData, error) {
	disbursementId := claim.RewardID + ":" + claim.Specifier
	authority, _, err := deriveAuthorityAccount(
		ProgramID,
		rc.rewardManagerStateAccount,
	)
	if err != nil {
		return nil, err
	}
	attestationsAccountAddress, _, err := deriveAttestationsAccount(
		ProgramID,
		authority,
		disbursementId,
	)
	if err != nil {
		return nil, err
	}
	attestationsData := AttestationsAccountData{}
	err = rc.client.GetAccountDataInto(
		ctx,
		attestationsAccountAddress,
		&attestationsData,
	)
	if err != nil {
		return nil, err
	}
	return &attestationsData, nil
}
