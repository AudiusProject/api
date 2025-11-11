/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sol_refresher",
	Short: "Utility to refresh Solana accounts",
	Long: `sol_refresher is a command line utility to refresh Solana accounts by
interacting with a Solana RPC to fetch related account data. It's most common
purpose is for backfilling accounts that were missed during indexing, 
especially after updates or additions to the indexer.

It leverages the sol_retry_queue instead of updating the db directly, letting 
the existing indexer infrastructure handle the updates. Thus, the refresh
doesn't happen immediately. They will be processed when the next retry job runs.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("rpc", "c", "https://api.mainnet-beta.solana.com", "The Solana RPC endpoint to use")
}
