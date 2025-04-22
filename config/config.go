package config

import (
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
	if os.Getenv("ENV") == "stage" {
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.staging.audius.co",
			"https://discoveryprovider2.staging.audius.co",
			"https://discoveryprovider3.staging.audius.co",
			"https://discoveryprovider5.staging.audius.co",
		}
	} else {
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.audius.co",
			"https://discoveryprovider2.audius.co",
			"https://discoveryprovider3.audius.co",
		}
	}
}
