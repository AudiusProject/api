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
	WriteDbUrl                     string
	RunMigrations                  bool
	EsUrl                          string
	Nodes                          []Node
	ArtistCoinRewardsStaticSenders []Node
	StoreAllNodes                  []string
	DeadNodes                      []string
	UploadNodes                    []string
	VerifierAddress                string
	DelegatePrivateKey             string
	AxiomToken                     string
	AxiomDataset                   string
	NetworkTakeRate                float64
	SolanaConfig                   SolanaConfig
	AntiAbuseOracles               []string
	Rewards                        []rewards.Reward
	AudiusdURL                     string
	OpenAudioURLs                  []string
	ChainId                        string
	BirdeyeToken                   string
	SolanaIndexerWorkers           int
	SolanaIndexerRetryInterval     time.Duration
	CommsMessagePush               bool
	AudiusdChainID                 uint
	AudiusdEntityManagerAddress    string
	AudiusAppUrl                   string
	ContentNodeMonitor             bool
	RewardCodeAuthorizedKeys       []string
	LaunchpadDeterministicSecret   string
}

var Cfg = Config{
	Git:                          os.Getenv("GIT_SHA"),
	Env:                          os.Getenv("ENV"),
	LogLevel:                     os.Getenv("logLevel"),
	ReadDbUrl:                    os.Getenv("readDbUrl"),
	ReadDbReplicas:               strings.Split(os.Getenv("readDbReplicas"), ","),
	WriteDbUrl:                   os.Getenv("writeDbUrl"),
	RunMigrations:                os.Getenv("runMigrations") == "true",
	EsUrl:                        os.Getenv("elasticsearchUrl"),
	DelegatePrivateKey:           os.Getenv("delegatePrivateKey"),
	AxiomToken:                   os.Getenv("axiomToken"),
	AxiomDataset:                 os.Getenv("axiomDataset"),
	NetworkTakeRate:              10,
	AudiusdURL:                   os.Getenv("audiusdUrl"),
	OpenAudioURLs:                []string{},
	BirdeyeToken:                 os.Getenv("birdeyeToken"),
	SolanaIndexerWorkers:         50,
	SolanaIndexerRetryInterval:   5 * time.Minute,
	CommsMessagePush:             true,
	ContentNodeMonitor:           true,
	LaunchpadDeterministicSecret: os.Getenv("launchpadDeterministicSecret"),
}

func init() {
	// Parse zap level from config
	zapLevel, err := zapcore.ParseLevel(Cfg.LogLevel)
	if err != nil {
		zapLevel = zapcore.InfoLevel
	}
	Cfg.ZapLevel = zapLevel

	Cfg.SolanaConfig = NewSolanaConfig()

	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		Cfg.OpenAudioURLs = []string{
			"http://audius-creator-node-1",
			"http://audius-creator-node-2",
			"http://audius-creator-node-3",
		}
		if Cfg.DelegatePrivateKey == "" {
			// Dummy key
			Cfg.DelegatePrivateKey = "13422b9affd75ff80f94f1ea394e6a6097830cb58cda2d3542f37464ecaee7df"
		}
		Cfg.AntiAbuseOracles = []string{"http://audius-discovery-provider-1"}
		Cfg.Nodes = DevNodes
		Cfg.Rewards = core_config.MakeRewards(core_config.DevClaimAuthorities, core_config.DevRewardExtensions)
		Cfg.AudiusdURL = "http://audius-creator-node-1"
		Cfg.ChainId = "audius-devnet"
		Cfg.SolanaIndexerWorkers = 1
		Cfg.DeadNodes = []string{}
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
		Cfg.Nodes = StageNodes
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
				DelegateOwnerWallet: "0x140eD283b33be2145ed7d9d15f1fE7bF1E0B2Ac3",
				Endpoint:            "https://creatornode9.staging.audius.co",
				OwnerWallet:         "0x140eD283b33be2145ed7d9d15f1fE7bF1E0B2Ac3",
			},
			{
				DelegateOwnerWallet: "0x4c88d2c0f4c4586b41621aD6e98882ae904B98f6",
				Endpoint:            "https://creatornode11.staging.audius.co",
				OwnerWallet:         "0x4c88d2c0f4c4586b41621aD6e98882ae904B98f6",
			},
			{
				DelegateOwnerWallet: "0x6b52969934076318863243fb92E9C4b3A08267b5",
				Endpoint:            "https://creatornode12.staging.audius.co",
				OwnerWallet:         "0x6b52969934076318863243fb92E9C4b3A08267b5",
			},
		}
	case "prod":
		fallthrough
	case "production":
		Cfg.OpenAudioURLs = []string{
			"cn1.shakespearetech.com",
			"audius-content-1.figment.io",
			"cn1.stuffisup.com",
			"audius-content-1.cultur3stake.com",
		}
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.audius.co"}
		Cfg.Nodes = ProdNodes
		Cfg.DeadNodes = []string{
			"https://content.grassfed.network",
		}
		Cfg.StoreAllNodes = []string{
			"https://creatornode2.audius.co",
		}
		Cfg.UploadNodes = ProdUploadNodes
		Cfg.Rewards = core_config.MakeRewards(core_config.ProdClaimAuthorities, core_config.ProdRewardExtensions)
		Cfg.AudiusdURL = "creatornode.audius.co"
		Cfg.ChainId = "audius-mainnet-alpha-beta"
		Cfg.AudiusdChainID = core_config.ProdAcdcChainID
		Cfg.AudiusdEntityManagerAddress = core_config.ProdAcdcAddress
		Cfg.AudiusAppUrl = "https://audius.co"
		Cfg.RewardCodeAuthorizedKeys = []string{"4oGhuh6MkypUTnwUzKbtnUwFzjfaMWAgKYudchPfbYu8", "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"}
		Cfg.VerifierAddress = "0xbeef8E42e8B5964fDD2b7ca8efA0d9aef38AA996"
		Cfg.ArtistCoinRewardsStaticSenders = []Node{
			{
				DelegateOwnerWallet: "0xc8d0C29B6d540295e8fc8ac72456F2f4D41088c8",
				Endpoint:            "https://creatornode.audius.co",
				OwnerWallet:         "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d",
			},
			{
				DelegateOwnerWallet: "0x159200F84c2cF000b3A014cD4D8244500CCc36ca",
				Endpoint:            "https://audius-cn1.tikilabs.com",
				OwnerWallet:         "0xe4882D9A38A2A1fc652996719AF0fb15CB968d0a",
			},
			{
				DelegateOwnerWallet: "0x627d23D17a3eAaDB1D3823e73Ab80D474023Acab",
				Endpoint:            "https://audius.bragi.cc",
				OwnerWallet:         "0xC88C8F9a15453c7D8Ea83120Af54cc4C40EC094a",
			},
			{
				DelegateOwnerWallet: "0x422541273087beC833c57D3c15B9e17F919bFB1F",
				Endpoint:            "https://v.monophonic.digital",
				OwnerWallet:         "0x6470Daf3bd32f5014512bCdF0D02232f5640a5BD",
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
}
