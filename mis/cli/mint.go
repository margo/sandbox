package cli

// cli/mint.go
// Defines the "mint" command and its subcommands for generating SVIDs.
// Currently supports the "x509" subcommand for minting X.509 SVIDs.
// The structure is designed to allow additional subcommands (e.g., "jwt") in the future.

import (
	"github.com/spf13/cobra"
)

// mintCmd represents the "mint" command.
// It acts as a parent command for SVID minting operations.
// Subcommands (e.g., x509) are registered under this command.
var mintCmd = &cobra.Command{
	Use:   "mint",
	Short: "Mint a new SVID",
	Long: `Mint generates a new SPIFFE Verifiable Identity Document (SVID).

Available subcommands allow minting different types of SVIDs:
  - x509 : Mint an X.509 SVID

Use "svidctl mint [subcommand] --help" for more information about a subcommand.

Examples:
  # Mint an X.509 SVID
  svidctl mint x509 --spiffeID spiffe://example.org/myservice --outputDir /tmp/svid`,
}

func init() {
	// Register x509 as a subcommand of mint.
	// Additional subcommands (e.g., jwt) can be added here in the future.
	mintCmd.AddCommand(x509Cmd)
}
