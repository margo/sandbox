package cmd

import (
	"fmt"
	"os"

	"github.com/eclipse-symphony/symphony/cli/utils"
	margoClientPkg "github.com/margo/dev-repo/sdk/client"
	"github.com/spf13/cobra"
)

var (
	appPackagePath             string
	applicationDescriptionFile string
	applicationPackageGitURL   string
	target                     string
)

var MargoCmd = &cobra.Command{
	Use:   "margo",
	Short: "Margo commands",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\n%sOnly 'margo onboardapp' command is supported as of now%s\n\n", utils.ColorRed(), utils.ColorReset())
	},
}

var MargoOnboardApp = &cobra.Command{
	Use:   "onboardapp",
	Short: "To onboard a margo application description",
	Run: func(cmd *cobra.Command, args []string) {
		if err := validateOnboardFlags(); err != nil {
			fmt.Printf("\n%s%s%s\n\n", utils.ColorRed(), err.Error(), utils.ColorReset())
			return
		}

		northboundClient := margoClientPkg.NewNorthboundClient(nil, nil)
		if err := onboardApplication(northboundClient); err != nil {
			fmt.Printf("\n%sOnboarding failed: %s%s\n\n", utils.ColorRed(), err.Error(), utils.ColorReset())
			return
		}

		fmt.Printf("\n%sApplication onboarded successfully%s\n\n", utils.ColorGreen(), utils.ColorReset())
	},
}

func validateOnboardFlags() error {
	if applicationPackageGitURL == "" && applicationDescriptionFile == "" {
		return fmt.Errorf("either --url or --file must be provided")
	}
	if applicationPackageGitURL != "" && applicationDescriptionFile != "" {
		return fmt.Errorf("only one of --url or --file can be provided")
	}
	return nil
}

func onboardApplication(client *margoClientPkg.NorthboundClient) error {
	var err error

	// Parse application from file or Git URL
	if applicationDescriptionFile != "" {
		fileHandler, err := os.Open(applicationDescriptionFile)
		if err != nil {
			return fmt.Errorf("failed to open application description file: %w", err)
		}
		defer fileHandler.Close()

		err = client.OnboardApplicationPackage(margoClientPkg.OnboardApplicationPackageOptions{
			Reader: fileHandler,
		})
	}

	if applicationPackageGitURL != "" {
		err = client.OnboardApplicationPackage(margoClientPkg.OnboardApplicationPackageOptions{
			GitURL: applicationPackageGitURL,
		})
	}

	if err != nil {
		return fmt.Errorf("failed to onboard application: %w", err)
	}

	return nil
}

func init() {
	// Onboard app flags
	MargoOnboardApp.Flags().StringVar(&applicationPackageGitURL, "url", "", "Git repository URL that has Margo Application definition")
	MargoOnboardApp.Flags().StringVarP(&applicationDescriptionFile, "file", "f", "", "Margo application description file path")

	// Add commands
	MargoCmd.AddCommand(MargoOnboardApp)
	RootCmd.AddCommand(MargoCmd)
}
