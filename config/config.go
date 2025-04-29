package config

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Env                           string
	DbUrl                         string
	Nodes                         []Node
	DeadNodes                     []string
	DelegatePrivateKey            string
	AxiomToken                    string
	AxiomDataset                  string
	PythonUpstreams               []string
	NetworkTakeRate               float64
	StakingBridgeUsdcPayoutWallet string
	SolanaConfig                  SolanaConfig
	AntiAbuseOracles              []string
}

var Cfg = Config{
	Env:                           os.Getenv("ENV"),
	DbUrl:                         os.Getenv("discoveryDbUrl"),
	DelegatePrivateKey:            os.Getenv("delegatePrivateKey"),
	AxiomToken:                    os.Getenv("axiomToken"),
	AxiomDataset:                  os.Getenv("axiomDataset"),
	NetworkTakeRate:               10,
	StakingBridgeUsdcPayoutWallet: "7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf",
}

func init() {
	Cfg.SolanaConfig = NewSolanaConfig()

	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		Cfg.AntiAbuseOracles = []string{"http://audius-protocol-discovery-provider-1"}
		Cfg.Nodes = DevNodes
		// Dummy key
		Cfg.DelegatePrivateKey = "13422b9affd75ff80f94f1ea394e6a6097830cb58cda2d3542f37464ecaee7df"
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
	default:
		log.Fatalf("Unknown environment: %s", env)
	}
}
