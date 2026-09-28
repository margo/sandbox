// main.go
// Entry point for the SVID management CLI application.
// This application provides commands to manage SPIFFE Verifiable Identity Documents (SVIDs).
package main

import (
	"github.com/margo/sandbox/mis/cli"
)

func main() {
	cli.Execute()
}
