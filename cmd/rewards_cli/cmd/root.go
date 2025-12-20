package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rewards_cli",
	Short: "Manages the Reward Manager program's registered senders and related lookup table",
}

func init() {
	rootCmd.PersistentFlags().StringP("rpc", "r", "", "Solana RPC endpoint (overrides env default)")
	rootCmd.PersistentFlags().StringP("openAudioURL", "o", "", "OpenAudio SDK URL (overrides env default)")
	rootCmd.PersistentFlags().StringP("keypair", "k", "", "Fee payer keypair file (overrides env default)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Dry run mode - check accounts without registering")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
