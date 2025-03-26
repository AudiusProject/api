package main

import (
	"fmt"

	"bridgerton.audius.co/api"
)

func main() {
	fmt.Println("hello bridgerton")
	as := api.NewApiServer()
	as.Serve()
}
