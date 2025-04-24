package config

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Env                           string
	DbUrl                         string
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
	SolanaConfig:                  SolCfg,
}

func init() {
	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		Cfg.AntiAbuseOracles = []string{"http://audius-protocol-discovery-provider-1"}
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
	default:
		log.Fatalf("Unknown environment: %s", env)
	}
}
