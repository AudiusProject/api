package config

import (
	"log"
	"os"
	"strings"

	"bridgerton.audius.co/api/spl/programs/claimable_tokens"
	"bridgerton.audius.co/api/spl/programs/reward_manager"
	"github.com/gagliardetto/solana-go"
)

type SolanaConfig struct {
	RpcProviders []string
	FeePayers    []solana.Wallet
	SolanaRelay  string

	MintAudio solana.PublicKey

	RewardManagerProgramID   solana.PublicKey
	RewardManagerState       solana.PublicKey
	RewardManagerLookupTable solana.PublicKey

	ClaimableTokensProgramID solana.PublicKey
}

const (
	// Dev
	DevSolanaRelay              = "http://audius-protocol-discovery-provider-1/solana/relay"
	DevMintAudio                = "37RCjhgV1qGV2Q54EHFScdxZ22ydRMdKMtVgod47fDP3"
	DevRewardManagerProgramID   = "testLsJKtyABc9UXJF8JWFKf1YH4LmqCWBC42c6akPb"
	DevRewardManagerState       = "DJPzVothq58SmkpRb1ATn5ddN2Rpv1j2TcGvM3XsHf1c"
	DevRewardManagerLookupTable = "GNHKVSmHvoRBt1JJCxz7RSMfzDQGDGhGEjmhHyxb3K5J"
	DevClaimableTokensProgramID = "testHKV1B56fbvop4w6f2cTGEub9dRQ2Euta5VmqdX9"

	// Stage
	StageSolanaRelay              = "https://discoveryprovider.staging.audius.co/solana/relay"
	StageMintAudio                = "BELGiMZQ34SDE6x2FUaML2UHDAgBLS64xvhXjX5tBBZo"
	StageRewardManagerProgramID   = "CDpzvz7DfgbF95jSSCHLX3ERkugyfgn9Fw8ypNZ1hfXp"
	StageRewardManagerState       = "GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq"
	StageRewardManagerLookupTable = "ChFCWjeFxM6SRySTfT46zXn2K7m89TJsft4HWzEtkB4J"
	StageClaimableTokensProgramID = "2sjQNmUfkV6yKKi4dPR8gWRgtyma5aiymE3aXL2RAZww"

	// Prod
	ProdSolanaRelay              = "https://discoveryprovider.audius.co/solana/relay"
	ProdMintAudio                = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
	ProdRewardManagerProgramID   = "DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei"
	ProdRewardManagerState       = "71hWFVYokLaN1PNYzTAWi13EfJ7Xt9VbSWUKsXUT8mxE"
	ProdRewardManagerLookupTable = "4UQwpGupH66RgQrWRqmPM9Two6VJEE68VZ7GeqZ3mvVv"
	ProdClaimableTokensProgramID = "Ewkv3JahEFRKkcJmpoKB7pXbnUHwjAyXiwEo4ZY2rezQ"
)

func NewSolanaConfig() SolanaConfig {
	cfg := SolanaConfig{
		RpcProviders: strings.Split(os.Getenv("solanaRpcProviders"), ","),
	}
	keyString := os.Getenv("solanaFeePayerKeys")
	if keyString != "" {
		walletKeys := strings.Split(keyString, ",")
		cfg.FeePayers = make([]solana.Wallet, len(walletKeys))
		for i, privkeyString := range walletKeys {
			privkey := solana.MustPrivateKeyFromBase58(privkeyString)
			cfg.FeePayers[i] = solana.Wallet{
				PrivateKey: privkey,
			}
		}
	} else {
		cfg.FeePayers = make([]solana.Wallet, 0)
	}

	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		cfg.SolanaRelay = DevSolanaRelay
		cfg.MintAudio = solana.MustPublicKeyFromBase58(DevMintAudio)
		cfg.RewardManagerProgramID = solana.MustPublicKeyFromBase58(DevRewardManagerProgramID)
		cfg.RewardManagerState = solana.MustPublicKeyFromBase58(DevRewardManagerState)
		cfg.RewardManagerLookupTable = solana.MustPublicKeyFromBase58(DevRewardManagerLookupTable)
		cfg.ClaimableTokensProgramID = solana.MustPublicKeyFromBase58(DevClaimableTokensProgramID)
	case "stage":
		fallthrough
	case "staging":
		cfg.SolanaRelay = StageSolanaRelay
		cfg.MintAudio = solana.MustPublicKeyFromBase58(StageMintAudio)
		cfg.RewardManagerProgramID = solana.MustPublicKeyFromBase58(StageRewardManagerProgramID)
		cfg.RewardManagerState = solana.MustPublicKeyFromBase58(StageRewardManagerState)
		cfg.RewardManagerLookupTable = solana.MustPublicKeyFromBase58(StageRewardManagerLookupTable)
		cfg.ClaimableTokensProgramID = solana.MustPublicKeyFromBase58(StageClaimableTokensProgramID)
	case "prod":
		fallthrough
	case "production":
		cfg.SolanaRelay = ProdSolanaRelay
		cfg.MintAudio = solana.MustPublicKeyFromBase58(ProdMintAudio)
		cfg.RewardManagerProgramID = solana.MustPublicKeyFromBase58(ProdRewardManagerProgramID)
		cfg.RewardManagerState = solana.MustPublicKeyFromBase58(ProdRewardManagerState)
		cfg.RewardManagerLookupTable = solana.MustPublicKeyFromBase58(ProdRewardManagerLookupTable)
		cfg.ClaimableTokensProgramID = solana.MustPublicKeyFromBase58(ProdClaimableTokensProgramID)
	default:
		log.Fatalf("Unknown environment: %s", env)
	}

	reward_manager.SetProgramID(cfg.RewardManagerProgramID)
	claimable_tokens.SetProgramID(cfg.ClaimableTokensProgramID)
	return cfg
}
