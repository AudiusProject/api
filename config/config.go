package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	core_config "github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/rewards"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Env                            string
	Git                            string
	LogLevel                       string
	ZapLevel                       zapcore.Level
	ReadDbUrl                      string
	ReadDbReplicas                 []string
	ReadDbMaxConns                 int32
	WriteDbUrl                     string
	RunMigrations                  bool
	EsUrl                          string
	ArtistCoinRewardsStaticSenders []Node
	VerifierAddress                string
	DelegatePrivateKey             string
	AxiomToken                     string
	AxiomDataset                   string
	NetworkTakeRate                float64
	SolanaConfig                   SolanaConfig
	AntiAbuseOracles               []string
	ArchiverNodes                  []string
	Rewards                        []rewards.Reward
	AudiusdURL                     string
	CoreBlockStreamEnabled         bool
	OpenAudioURLs                  []string
	ChainId                        string
	// HTTP(S) JSON-RPC endpoint for the Ethereum mainnet provider (e.g. an
	// Alchemy URL). Used by the eth-indexer for backfill `eth_getLogs` and
	// targeted `balanceOf` reads. If empty, the indexer is a no-op.
	EthRpcUrl string
	// WebSocket JSON-RPC endpoint used by the eth-indexer for live
	// subscriptions to AUDIO Transfer events. Auto-derived from EthRpcUrl
	// (https:// -> wss://) if left unset.
	EthWsUrl string
	// AUDIO ERC-20 contract address on Ethereum mainnet. Override only when
	// pointing at a non-mainnet deployment.
	EthAudioContractAddress string
	// Audius Staking proxy address — used to read totalStakedFor(holder).
	EthStakingContractAddress string
	// Audius DelegateManager address — used to read
	// getTotalDelegatorStake(holder).
	EthDelegateManagerContractAddress string
	SolanaIndexerWorkers              int
	SolanaIndexerRetryInterval        time.Duration
	CommsMessagePush                  bool
	AudiusdChainID                    uint
	AudiusdEntityManagerAddress       string
	AudiusAppUrl                      string
	RewardCodeAuthorizedKeys          []string
	LaunchpadDeterministicSecret      string
	UnsplashKeys                      []string
	// Nodes that volunteer as STORE_ALL nodes and are always included in mirrors lists
	StoreAllNodes []string
	// Nodes that are truly dead and should not be included in rendezvous
	DeadNodes []string
	// Nodes that are blacklisted and should not be included in mirrors lists
	BlacklistedNodes []string
	// Nodes that should handle inbound uploads
	UploadNodes []string
	// Optional API secret to be used for api.audius.co frontends
	AudiusApiSecret string
	// Shared secret for notifications-dashboard (or other internal jobs) to read notification campaign push open counts
	NotificationCampaignOpenMetricsSecret string
	// Audius account user id. In the public remix-contests list, hosts followed
	// by this account sort ahead of other hosts within both the open and ended
	// groups. Zero (the default when the env var is unset) disables follow-based
	// prioritization (the list reduces to open-before-ended).
	FeaturedAudienceUserID int32

	// Genesis migration dual-write queue.
	// NewChainURL is the bootstrap chain gRPC endpoint (e.g. http://bootstrap-node:50051).
	// NewChainQueueEnabled turns on enqueuing of relayed txs for the new chain.
	// NewChainFlushEnabled turns on the background flusher goroutine.
	// NewChainFlushFromBlock, when set, causes the flusher to delete all queued rows with
	// confirmed_block < NewChainFlushFromBlock before sending — trimming rows already
	// covered by the backfill.
	// NewChainInsecureSkipVerify disables TLS verification for the new chain endpoint (e.g. localstack).
	NewChainURL                string
	NewChainQueueEnabled       bool
	NewChainFlushEnabled       bool
	NewChainFlushFromBlock     int64
	NewChainFlushToBlock       int64
	NewChainInsecureSkipVerify bool

	// Indexer cutover bounds. Both default to 0, meaning "unset" — the ETL
	// treats a zero start as "resume from where you left off" and a zero end as
	// "never stop", which is normal operation.
	//
	// They exist for the chain cutover, where the old-chain indexer must stop at
	// a known height L and the new-chain indexer must begin at a known height,
	// rather than resuming off MAX(block_height) — a query with no chain_id,
	// which against a fresh chain resolves to the old chain's height and stalls
	// silently. See cmd/genesis-writer/ROLLOUT.md, Runbook step 12.
	EtlStartingBlockHeight int64
	EtlEndingBlockHeight   int64
}

var Cfg = Config{
	Git:                                   os.Getenv("GIT_SHA"),
	Env:                                   os.Getenv("ENV"),
	LogLevel:                              os.Getenv("logLevel"),
	ReadDbUrl:                             os.Getenv("readDbUrl"),
	ReadDbReplicas:                        strings.Split(os.Getenv("readDbReplicas"), ","),
	ReadDbMaxConns:                        8,
	WriteDbUrl:                            os.Getenv("writeDbUrl"),
	RunMigrations:                         os.Getenv("runMigrations") == "true",
	EsUrl:                                 os.Getenv("elasticsearchUrl"),
	DelegatePrivateKey:                    os.Getenv("delegatePrivateKey"),
	AxiomToken:                            os.Getenv("axiomToken"),
	AxiomDataset:                          os.Getenv("axiomDataset"),
	NetworkTakeRate:                       10,
	AudiusdURL:                            os.Getenv("audiusdUrl"),
	CoreBlockStreamEnabled:                os.Getenv("coreBlockStreamEnabled") == "true",
	OpenAudioURLs:                         []string{},
	EthRpcUrl:                             os.Getenv("ethRpcUrl"),
	EthWsUrl:                              os.Getenv("ethWsUrl"),
	EthAudioContractAddress:               os.Getenv("ethAudioContractAddress"),
	EthStakingContractAddress:             os.Getenv("ethStakingContractAddress"),
	EthDelegateManagerContractAddress:     os.Getenv("ethDelegateManagerContractAddress"),
	SolanaIndexerWorkers:                  50,
	SolanaIndexerRetryInterval:            5 * time.Minute,
	CommsMessagePush:                      true,
	LaunchpadDeterministicSecret:          os.Getenv("launchpadDeterministicSecret"),
	UnsplashKeys:                          strings.Split(os.Getenv("unsplashKeys"), ","),
	AudiusApiSecret:                       os.Getenv("audiusApiSecret"),
	NotificationCampaignOpenMetricsSecret: os.Getenv("notificationCampaignOpenMetricsSecret"),
}

func init() {
	// Parse zap level from config
	zapLevel, err := zapcore.ParseLevel(Cfg.LogLevel)
	if err != nil {
		zapLevel = zapcore.InfoLevel
	}
	Cfg.ZapLevel = zapLevel

	Cfg.SolanaConfig = NewSolanaConfig()

	// Default AUDIO ERC-20 + staking + delegate manager to mainnet addresses
	// (from packages/sdk/src/sdk/config/production.ts).
	if Cfg.EthAudioContractAddress == "" {
		Cfg.EthAudioContractAddress = "0x18aAA7115705e8be94bfFEbDE57Af9BFc265B998"
	}
	if Cfg.EthStakingContractAddress == "" {
		Cfg.EthStakingContractAddress = "0xe6D97B2099F142513be7A2a068bE040656Ae4591"
	}
	if Cfg.EthDelegateManagerContractAddress == "" {
		Cfg.EthDelegateManagerContractAddress = "0x4d7968ebfD390D5E7926Cb3587C39eFf2F9FB225"
	}
	// Derive WS endpoint from the HTTP endpoint if not set explicitly.
	if Cfg.EthWsUrl == "" && Cfg.EthRpcUrl != "" {
		switch {
		case strings.HasPrefix(Cfg.EthRpcUrl, "https://"):
			Cfg.EthWsUrl = "wss://" + strings.TrimPrefix(Cfg.EthRpcUrl, "https://")
		case strings.HasPrefix(Cfg.EthRpcUrl, "http://"):
			Cfg.EthWsUrl = "ws://" + strings.TrimPrefix(Cfg.EthRpcUrl, "http://")
		}
	}

	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		Cfg.OpenAudioURLs = []string{
			"https://node1.oap.devnet",
			"https://node2.oap.devnet",
			"https://node3.oap.devnet",
			"https://node4.oap.devnet",
		}
		if Cfg.DelegatePrivateKey == "" {
			// Dummy key
			Cfg.DelegatePrivateKey = "13422b9affd75ff80f94f1ea394e6a6097830cb58cda2d3542f37464ecaee7df"
		}
		Cfg.AntiAbuseOracles = []string{"http://audius-discovery-provider-1"}
		Cfg.ArchiverNodes = []string{"https://archiver.audius.engineering"}
		Cfg.Rewards = core_config.MakeRewards(core_config.DevClaimAuthorities, core_config.DevRewardExtensions)
		Cfg.AudiusdURL = "https://node1.oap.devnet"
		Cfg.ChainId = "openaudio-devnet"
		Cfg.SolanaIndexerWorkers = 1
		Cfg.DeadNodes = []string{}
		Cfg.BlacklistedNodes = []string{}
		Cfg.StoreAllNodes = []string{}
		Cfg.UploadNodes = DevUploadNodes
		Cfg.AudiusdChainID = core_config.DevAcdcChainID
		Cfg.AudiusdEntityManagerAddress = core_config.DevAcdcAddress
		Cfg.AudiusAppUrl = "http://localhost:3000"
	case "stage":
		fallthrough
	case "staging":
		Cfg.OpenAudioURLs = []string{
			"creatornode11.staging.audius.co",
			"creatornode5.staging.audius.co",
			"creatornode12.staging.audius.co",
		}
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.staging.audius.co"}
		Cfg.ArchiverNodes = []string{"https://discoveryprovider.staging.audius.co"}
		Cfg.DeadNodes = []string{}
		Cfg.StoreAllNodes = []string{}
		Cfg.UploadNodes = StageUploadNodes
		Cfg.Rewards = core_config.MakeRewards(core_config.StageClaimAuthorities, core_config.StageRewardExtensions)
		Cfg.AudiusdURL = "creatornode11.staging.audius.co"
		Cfg.ChainId = "audius-testnet-alpha"

		Cfg.AudiusdChainID = core_config.StageAcdcChainID
		Cfg.AudiusdEntityManagerAddress = core_config.StageAcdcAddress
		Cfg.AudiusAppUrl = "https://staging.audius.co"
		Cfg.RewardCodeAuthorizedKeys = []string{"9XeZbswbSSUU4AHVArQbTQjAEjAPhVweGU5cogBVkvh4", "GrWNH9qfwrvoCEoTm65hmnSh4z3CD96SfhtfQY6ZKUfY"}
		Cfg.VerifierAddress = "0xbbbb93A6B3A1D6fDd27909729b95CCB0cc9002C0"
		Cfg.ArtistCoinRewardsStaticSenders = []Node{
			{
				DelegateWallet: "0x140eD283b33be2145ed7d9d15f1fE7bF1E0B2Ac3",
				Endpoint:       "https://creatornode9.staging.audius.co",
				Owner:          "0x140eD283b33be2145ed7d9d15f1fE7bF1E0B2Ac3",
			},
			{
				DelegateWallet: "0x4c88d2c0f4c4586b41621aD6e98882ae904B98f6",
				Endpoint:       "https://creatornode11.staging.audius.co",
				Owner:          "0x4c88d2c0f4c4586b41621aD6e98882ae904B98f6",
			},
			{
				DelegateWallet: "0x6b52969934076318863243fb92E9C4b3A08267b5",
				Endpoint:       "https://creatornode12.staging.audius.co",
				Owner:          "0x6b52969934076318863243fb92E9C4b3A08267b5",
			},
		}
	case "prod":
		fallthrough
	case "production":
		Cfg.OpenAudioURLs = []string{
			"rpc.audius.co",
		}
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://anti-abuse-oracle.audius.engineering"}
		Cfg.ArchiverNodes = []string{"https://archiver.audius.engineering"}
		Cfg.DeadNodes = []string{
			"https://content.grassfed.network",
		}
		Cfg.BlacklistedNodes = []string{
			"https://audius-discovery-2.cultur3stake.com",
			"https://audius-discovery-3.cultur3stake.com",
			"https://audius-discovery-4.cultur3stake.com",
			"https://audius-discovery-7.cultur3stake.com",
			"https://audius-discovery-8.cultur3stake.com",
			"https://audius-discovery-10.cultur3stake.com",
			"https://audius-discovery-1.cultur3stake.com",
			"https://audius-content-1.cultur3stake.com",
			"https://audius-content-2.cultur3stake.com",
			"https://audius-content-3.cultur3stake.com",
			"https://audius-content-4.cultur3stake.com",
			"https://audius-content-5.cultur3stake.com",
			"https://audius-content-6.cultur3stake.com",
			"https://audius-content-7.cultur3stake.com",
			"https://audius-content-8.cultur3stake.com",
			"https://audius-content-9.cultur3stake.com",
			"https://audius-content-10.cultur3stake.com",
			"https://audius-content-11.cultur3stake.com",
			"https://audius-content-12.cultur3stake.com",
			"https://audius-content-13.cultur3stake.com",
		}
		Cfg.StoreAllNodes = []string{
			"https://v.monophonic.digital",
		}
		Cfg.UploadNodes = ProdUploadNodes
		Cfg.Rewards = core_config.MakeRewards(core_config.ProdClaimAuthorities, core_config.ProdRewardExtensions)
		Cfg.AudiusdURL = "rpc.audius.co"
		Cfg.ChainId = "audius-mainnet-alpha-beta"
		Cfg.AudiusdChainID = core_config.ProdAcdcChainID
		Cfg.AudiusdEntityManagerAddress = core_config.ProdAcdcAddress
		Cfg.AudiusAppUrl = "https://audius.co"
		Cfg.RewardCodeAuthorizedKeys = []string{"4oGhuh6MkypUTnwUzKbtnUwFzjfaMWAgKYudchPfbYu8", "Du1sGwqC5yJFeoKJ73m3DrFLaRnc7rHM7g1z3Xe8jy8d", "4mzunYiiFSRGGc5iS3SVQQNdek6JiCyvkFALwoWu2xhP"}
		Cfg.VerifierAddress = "0xbeef8E42e8B5964fDD2b7ca8efA0d9aef38AA996"
		Cfg.ArtistCoinRewardsStaticSenders = []Node{
			{
				DelegateWallet: "0xc8d0C29B6d540295e8fc8ac72456F2f4D41088c8",
				Endpoint:       "https://creatornode.audius.co",
				Owner:          "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d",
			},
			{
				DelegateWallet: "0x627d23D17a3eAaDB1D3823e73Ab80D474023Acab",
				Endpoint:       "https://audius.bragi.cc",
				Owner:          "0xC88C8F9a15453c7D8Ea83120Af54cc4C40EC094a",
			},
			{
				DelegateWallet: "0x422541273087beC833c57D3c15B9e17F919bFB1F",
				Endpoint:       "https://v.monophonic.digital",
				Owner:          "0x6470Daf3bd32f5014512bCdF0D02232f5640a5BD",
			},
			{
				DelegateWallet: "0xae5d0507b6653589a03ae5becb35eb0c160e7131",
				Endpoint:       "https://audius.rickyrombo.com",
				Owner:          "0x923EC9976bfEcFd0E8b7fEeaC9115F740f8ddB00",
			},
		}
	default:
		log.Fatalf("Unknown environment: %s", env)
	}

	if os.Getenv("commsMessagePush") != "" {
		commsMessagePushEnabled, err := strconv.ParseBool(os.Getenv("commsMessagePush"))
		if err != nil {
			log.Fatalf("Invalid commsMessagePush: %s", err)
		}
		Cfg.CommsMessagePush = commsMessagePushEnabled
	}

	if v := os.Getenv("readDbMaxConns"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil || parsed <= 0 {
			log.Fatalf("Invalid readDbMaxConns %q: must be a positive integer", v)
		}
		Cfg.ReadDbMaxConns = int32(parsed)
	}

	// Solana indexer config
	retryInterval := os.Getenv("solanaIndexerRetryInterval")
	if retryInterval != "" {
		parsedInterval, err := time.ParseDuration(retryInterval)
		if err != nil {
			panic("Invalid solanaIndexerRetryInterval: " + err.Error())
		}
		Cfg.SolanaIndexerRetryInterval = parsedInterval
	}

	workers := os.Getenv("solanaIndexerWorkers")
	if workers != "" {
		parsedWorkers, err := strconv.Atoi(workers)
		if err != nil {
			panic("Invalid solanaIndexerWorkers: " + err.Error())
		}
		Cfg.SolanaIndexerWorkers = parsedWorkers
	}

	// Override archiver upstream(s) when set (e.g. rollback to discovery or point at different archiver)
	if v := os.Getenv("archiverNodes"); v != "" {
		Cfg.ArchiverNodes = strings.Split(v, ",")
	}

	// Override anti-abuse oracle endpoint(s) when set, so the URL can be rotated without a code deploy.
	if v := os.Getenv("antiAbuseOracles"); v != "" {
		Cfg.AntiAbuseOracles = strings.Split(v, ",")
	}

	// Override the Audius app base URL when set, so developer apps running against a
	// non-default frontend (local override, preview deploy) get the right redirect_uri base.
	if v := os.Getenv("audiusAppUrl"); v != "" {
		Cfg.AudiusAppUrl = strings.TrimSuffix(v, "/")
	}

	if v := os.Getenv("featuredAudienceUserId"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			log.Fatalf("Invalid featuredAudienceUserId: %s", err)
		}
		Cfg.FeaturedAudienceUserID = int32(parsed)
	}

	// Genesis migration dual-write queue
	Cfg.NewChainURL = os.Getenv("newChainUrl")
	Cfg.NewChainQueueEnabled = os.Getenv("newChainQueueEnabled") == "true"
	Cfg.NewChainFlushEnabled = os.Getenv("newChainFlushEnabled") == "true"
	Cfg.NewChainInsecureSkipVerify = os.Getenv("newChainInsecureSkipVerify") == "true"
	Cfg.NewChainFlushFromBlock = mustParseInt64Env("newChainFlushFromBlock")
	Cfg.NewChainFlushToBlock = mustParseInt64Env("newChainFlushToBlock")

	// Indexer cutover bounds (see the struct fields).
	Cfg.EtlStartingBlockHeight = mustParseInt64Env("etlStartingBlockHeight")
	Cfg.EtlEndingBlockHeight = mustParseInt64Env("etlEndingBlockHeight")
}

// mustParseInt64Env reads an optional int64 env var, returning 0 when unset.
// A malformed value panics rather than silently reading as 0: every caller here
// is a cutover bound where 0 means "no bound", so a typo would quietly disable
// the very limit it was meant to impose.
func mustParseInt64Env(name string) int64 {
	v := os.Getenv(name)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic("Invalid " + name + ": " + err.Error())
	}
	return n
}
