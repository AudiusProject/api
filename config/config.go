package config

import (
	"log"
	"os"

	core_config "github.com/AudiusProject/audiusd/pkg/core/config"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Env                string
	LogLevel           string
	ReadDbUrl          string
	WriteDbUrl         string
	RunMigrations      bool
	EsUrl              string
	Nodes              []Node
	DeadNodes          []string
	DelegatePrivateKey string
	AxiomToken         string
	AxiomDataset       string
	PythonUpstreams    []string
	NetworkTakeRate    float64
	SolanaConfig       SolanaConfig
	AntiAbuseOracles   []string
	Rewards            []rewards.Reward
	AudiusdURL         string
	ChainId            string
	BirdeyeToken       string
}

var Cfg = Config{
	Env:                os.Getenv("ENV"),
	LogLevel:           os.Getenv("logLevel"),
	ReadDbUrl:          os.Getenv("readDbUrl"),
	WriteDbUrl:         os.Getenv("writeDbUrl"),
	RunMigrations:      os.Getenv("runMigrations") == "true",
	EsUrl:              os.Getenv("elasticsearchUrl"),
	DelegatePrivateKey: os.Getenv("delegatePrivateKey"),
	AxiomToken:         os.Getenv("axiomToken"),
	AxiomDataset:       os.Getenv("axiomDataset"),
	NetworkTakeRate:    10,
	AudiusdURL:         os.Getenv("audiusdUrl"),
	BirdeyeToken:       os.Getenv("birdeyeToken"),
}

func init() {
	Cfg.SolanaConfig = NewSolanaConfig()

	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		if Cfg.DelegatePrivateKey == "" {
			// Dummy key
			Cfg.DelegatePrivateKey = "13422b9affd75ff80f94f1ea394e6a6097830cb58cda2d3542f37464ecaee7df"
		}
		Cfg.AntiAbuseOracles = []string{"http://audius-protocol-discovery-provider-1"}
		Cfg.Nodes = DevNodes
		Cfg.Rewards = core_config.MakeRewards(core_config.DevClaimAuthorities, core_config.DevRewardExtensions)
		Cfg.AudiusdURL = "http://audius-protocol-creator-node-1"
		Cfg.ChainId = "audius-mainnet-alpha-beta"
	case "stage":
		fallthrough
	case "staging":
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.staging.audius.co"}
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.staging.audius.co",
			"https://discoveryprovider2.staging.audius.co",
			"https://discoveryprovider3.staging.audius.co",
			"https://discoveryprovider5.staging.audius.co",
		}
		Cfg.Nodes = StageNodes
		Cfg.DeadNodes = []string{}
		Cfg.Rewards = core_config.MakeRewards(core_config.StageClaimAuthorities, core_config.StageRewardExtensions)
		Cfg.AudiusdURL = "creatornode11.staging.audius.co"
		Cfg.ChainId = "audius-testnet-alpha"
	case "prod":
		fallthrough
	case "production":
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.audius.co"}
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.audius.co",
			"https://discoveryprovider2.audius.co",
			"https://discoveryprovider3.audius.co",
		}
		Cfg.Nodes = ProdNodes
		Cfg.DeadNodes = []string{
			"https://content.grassfed.network",
		}
		Cfg.Rewards = core_config.MakeRewards(core_config.ProdClaimAuthorities, core_config.ProdRewardExtensions)
		Cfg.AudiusdURL = "creatornode.audius.co"
		Cfg.ChainId = "audius-mainnet-alpha-beta"
	default:
		log.Fatalf("Unknown environment: %s", env)
	}
}
