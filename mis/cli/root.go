package cli

// cli/root.go
// Package cli provides the CLI commands for the SVID management application.
// It uses the Cobra library to define and manage commands and subcommands.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command for the CLI application.
// All subcommands are registered under this root command.
var rootCmd = &cobra.Command{
	Use:   "svidctl",
	Short: "SVID management CLI",
	Long: `svidctl is a CLI tool for managing SPIFFE Verifiable Identity Documents (SVIDs).

It provides commands to:
  - Start a REST API server for SVID management
  - Mint new X.509 SVIDs with configurable parameters

Use "svidctl [command] --help" for more information about a command.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main() and only needs to happen once.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Register subcommands under root
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(mintCmd)
}
