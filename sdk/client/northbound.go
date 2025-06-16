package client

import (
	"fmt"
	"io"
	"time"

	"github.com/margo/dev-repo/sdk/auth"
	"github.com/margo/dev-repo/sdk/pkg/models"
	"github.com/margo/dev-repo/sdk/transport"
	"github.com/margo/dev-repo/sdk/utils"
)

type NorthboundClient struct {
	auth      auth.Authenticator
	transport transport.Transport
}

func NewNorthboundClient(auth auth.Authenticator, transport transport.Transport) *NorthboundClient {
	return &NorthboundClient{
		auth:      auth,
		transport: transport,
	}
}

func (client *NorthboundClient) onboardApplicationURI(appId string) string {
	return fmt.Sprintf("/github.com/margo/dev-repo/sdk/api/v1/application/%s", appId)
}

func (client *NorthboundClient) getApplicationURI(appId string) string {
	return fmt.Sprintf("/github.com/margo/dev-repo/sdk/api/v1/application/%s", appId)
}

func (client *NorthboundClient) listApplicationURI(appId string) string {
	return fmt.Sprintf("/github.com/margo/dev-repo/sdk/api/v1/application/%s", appId)
}

type OnboardApplicationPackageOptions struct {
	// Pass either the git config, or the ioreader
	// IOReader is given, in case if you already have a file, or a structure with you
	// you can pass an ioreader to that file, or structure by converting them to bytes, or filereader
	// the git is given in case you have a git repo where you have hosted the file

	// Git configuration (optional)
	GitURL  string
	GitAuth *utils.GitAuth
	Timeout time.Duration

	// Reader configuration (optional)
	Reader io.Reader
}

// this function should onboard an app package by reading from a git repo,
// or an ioReader, the user can pass any of them
func (client *NorthboundClient) OnboardApplicationPackage(option OnboardApplicationPackageOptions) error {
	if option.Reader == nil && option.GitURL == "" {
		return fmt.Errorf("either ioreader or git url should be provided")
	}

	var err error
	ioreader := option.Reader

	if ioreader == nil {
		ioreader, err = utils.ReadFromGitWithAuth(option.GitURL, option.GitAuth)
		if err != nil {
			return fmt.Errorf("failed to read from git: %w", err)
		}
	}

	applicationPackage, err := models.ParseApplication(ioreader)
	if err != nil {
		return fmt.Errorf("failed to parse application from io reader: %w", err)
	}
	// api endpoint of margo server (symphony or any wfm that has exposed margo apis)
	fmt.Println("ApplicationPacakge", applicationPackage)
	return nil
}

func (client *NorthboundClient) GetApplication() (*models.ApplicationDescription, error) {
	return &models.ApplicationDescription{}, nil
}

func (client *NorthboundClient) ListApplicationPackages() ([]models.ApplicationDescription, error) {
	return []models.ApplicationDescription{}, nil
}

func (client *NorthboundClient) DeleteApplicationPackage() error {
	return nil
}
