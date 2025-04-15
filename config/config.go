package config

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	DbUrl              string
	DelegatePrivateKey string
	AxiomToken         string
	AxiomDataset       string
}

var Cfg = Config{
	DbUrl:              os.Getenv("discoveryDbUrl"),
	DelegatePrivateKey: os.Getenv("delegatePrivateKey"),
	AxiomToken:         os.Getenv("axiomToken"),
	AxiomDataset:       os.Getenv("axiomDataset"),
}
