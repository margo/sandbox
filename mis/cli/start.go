package cli

// cli/start.go
// Defines the "start" command which launches the REST API server.
// The configuration file path is a required argument for this command.

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/margo/sandbox/mis/https"
	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/margo/sandbox/mis/pkg/log"
	"github.com/margo/sandbox/mis/unix"
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

		cnf, err := conf.LoadConfig(configFile)
		if err != nil {
			return err
		}
		logger := log.New(cnf.Log.Level)

		normativeServer := https.New(cnf, logger.With("server-type", "Normative"))
		mintServer := unix.New(cnf, logger.With("server-type", "Mint"))

		// For Graceful shutdown
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			if err := normativeServer.Start(); err != nil && err != http.ErrServerClosed {
				panic(fmt.Sprintf("normative server error: %v", err))
			}
		}()

		go func() {
			if err := mintServer.Start(); err != nil && err != http.ErrServerClosed {
				panic(fmt.Sprintf("mint server error: %v", err))
			}
		}()

		<-quit
		logger.Info("Shutting down normative server & mint server...")
		err = normativeServer.Stop()
		if err != nil {
			fmt.Printf(
				"Failed to shutdown normative server, probably force closed it, err: %s",
				err.Error(),
			)
		}

		err = mintServer.Stop()
		if err != nil {
			fmt.Printf(
				"Failed to shutdown mint server, probably force closed it, err: %s",
				err.Error(),
			)
		}

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
