package main

import (
	"fmt"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

// APIClient interface
type APIClientInterface interface {
	NewSBIClient() (sbi.ClientInterface, error)
}

// apiClient implementation
type apiClient struct {
	// You can add any dependencies needed for creating the client here, like auth info
	// clientId, clientSecret, tokenUrl string
	SBIUrl string
}

// NewClient creates a new sbi.Client
func (f *apiClient) NewSBIClient() (sbi.ClientInterface, error) {
	client, err := sbi.NewClient(f.SBIUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	client.RequestEditors = []sbi.RequestEditorFn{}
	return client, nil
}
