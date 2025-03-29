package main

import (
	"fmt"
	"os"

	"bridgerton.audius.co/api"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("hello bridgerton")
	as := api.NewApiServer(api.Config{
		DbUrl:        os.Getenv("discoveryDbUrl"),
		AxiomToken:   os.Getenv("axiomToken"),
		AxiomDataset: os.Getenv("axiomDataset"),
	})
	as.Serve()
}
