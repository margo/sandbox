package cli

// cli/start.go
// Defines the "start" command which launches the REST API server.
// The configuration file path is a required argument for this command.

import (
	"fmt"

	"github.com/spf13/cobra"
)

// configFile holds the path to the configuration file provided via --config flag.
var configFile string

// startCmd represents the "start" command.
// It starts the REST API server using the provided configuration file.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the REST API server",
	Long: `Start the REST API server for SVID management.

The server requires a configuration file to be specified using the --config flag.
The configuration file contains server settings such as port, TLS configuration,
and SPIRE agent connection details.

Examples:
  # Start the server with a configuration file
  svidctl start --config /etc/svidctl/config.yaml

  # Start the server with a configuration file in the current directory
  svidctl start --config ./config.yaml`,

	// RunE executes the start command logic, returning an error if execution fails.
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate that the config file path is not empty
		// (cobra marks it required, but this is an extra safety check)
		if configFile == "" {
			return fmt.Errorf("configuration file path cannot be empty")
		}

		fmt.Printf("Starting REST API server with config: %s\n", configFile)

		// TODO: Integrate server startup logic here
		// cnf, err := conf.LoadConfig(configFile)
		// if err != nil {
		// 	return err
		// }
		// logger := log.New(cnf.Log.Level)

		// Example: server.Start(cnf, logger)

		return nil
	},
}

func init() {
	// --config flag: path to the configuration file (required)
	startCmd.Flags().StringVarP(
		&configFile,
		"config", "c",
		"",
		"Path to the configuration file (required)",
	)

	// Mark --config as a required flag; cobra will enforce this automatically
	if err := startCmd.MarkFlagRequired("config"); err != nil {
		panic(fmt.Sprintf("failed to mark 'config' flag as required: %v", err))
	}
}
