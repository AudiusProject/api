package main

import (
	"fmt"

	"bridgerton.audius.co/api"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("hello bridgerton")
	as := api.NewApiServer()
	as.Serve()
}
