// Package client provides a CLI client for interacting with the Margo Northbound API.
//
// This package offers a high-level interface for managing application packages through
// the Margo Northbound service, including operations for onboarding, listing, retrieving,
// and deleting application packages.
package wfm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"
	"crypto/tls"
    "net/http"
	nonStdWfmNbi "github.com/margo/dev-repo/non-standard/generatedCode/wfm/nbi"
)

const (
	// northboundBaseURL is the default base URL path for the Northbound API
	northboundBaseURL = "margo/nbi/v1"

	// Default timeout for API requests
	nbiDefaultTimeout = 30 * time.Second
)

// Type aliases for better API ergonomics, and can be used later on to change the structs if needed
type (
	AppPkgOnboardingReq  = nonStdWfmNbi.ApplicationPackageManifestRequest
	AppPkgOnboardingResp = nonStdWfmNbi.ApplicationPackageManifestResp
	AppPkgSummary        = nonStdWfmNbi.ApplicationPackageManifestResp
	ListAppPkgsParams    = nonStdWfmNbi.ListAppPackagesParams
	ListAppPkgsResp      = nonStdWfmNbi.ApplicationPackageListResp

	DeploymentReq        = nonStdWfmNbi.ApplicationDeploymentManifestRequest
	DeploymentResp       = nonStdWfmNbi.ApplicationDeploymentManifestResp
	DeploymentListResp   = nonStdWfmNbi.ApplicationDeploymentListResp
	DeploymentListParams = nonStdWfmNbi.ListApplicationDeploymentsParams

	DeviceListResp = nonStdWfmNbi.DeviceListResp
)

// NbiApiClient provides a client interface for the Margo Northbound API.
//
// This client handles HTTP communication with the Northbound service and provides
// high-level methods for application package management operations.
type NbiApiClient struct {
	serverAddress string
	nbiBaseURL    string
	sbiBaseURL    string
	timeout       time.Duration
	logger        *log.Logger
	httpClient    *http.Client
}

// WFMCliOption defines functional options for configuring the client
type WFMCliOption func(*NbiApiClient)

// WithTimeout sets a custom timeout for API requests
func WithTimeout(timeout time.Duration) WFMCliOption {
	return func(cli *NbiApiClient) {
		cli.timeout = timeout
	}
}

// WithInsecureTLS configures the client to skip TLS verification (development only)
func WithInsecureTLS() WFMCliOption {
    return func(cli *NbiApiClient) {
        cli.httpClient = &http.Client{
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    InsecureSkipVerify: true, // Only for development
                },
            },
            Timeout: cli.timeout,
        }
    }
}


// WithLogger sets a custom logger for the client
func WithLogger(logger *log.Logger) WFMCliOption {
	return func(cli *NbiApiClient) {
		cli.logger = logger
	}
}

func WithAuth() WFMCliOption {
	return func(cli *NbiApiClient) {
	}
}

// NewNbiHTTPCli creates a new Northbound API client.
//
// Parameters:
//   - host: The hostname or IP address of the Northbound service
//   - port: The port number of the Northbound service
//   - basePath: Optional custom base path (uses default if nil)
//   - opts: Optional configuration functions
//
// Returns:
//   - *WFMCli: A configured client instance
//
// Example:
//
//	cli := NewNbiHTTPCli("localhost", 8080, nil,
//	    WithTimeout(60*time.Second),
//	    WithLogger(customLogger))
func NewNbiHTTPCli(host string, port uint16, nbiBasePath *string, opts ...WFMCliOption) *NbiApiClient {
    nbiBaseURLPath := northboundBaseURL
    if nbiBasePath != nil {
        nbiBaseURLPath = *nbiBasePath
    }

    cli := &NbiApiClient{
        serverAddress: fmt.Sprintf("%s:%d", host, port),
        nbiBaseURL:    fmt.Sprintf("https://%s:%d/%s", host, port, nbiBaseURLPath), // Changed to https
        timeout:       nbiDefaultTimeout,
        logger:        log.Default(),
        httpClient:    &http.Client{Timeout: nbiDefaultTimeout}, // Add default HTTP client
    }

    // Apply options
    for _, opt := range opts {
        opt(cli)
    }

    return cli
}


// createClient creates a new API client with proper error handling
func (cli *NbiApiClient) createNonStdNbiClient() (*nonStdWfmNbi.Client, error) {
    client, err := nonStdWfmNbi.NewClient(cli.nbiBaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to create API client: %w", err)
    }
    
    // Configure the client to use our custom HTTP client
    if cli.httpClient != nil {
        client.Client = cli.httpClient
    }
    
    return client, nil
}


// createContext creates a context with timeout
func (cli *NbiApiClient) createContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cli.timeout)
}

// handleErrorResponse processes error responses consistently
func (cli *NbiApiClient) handleErrorResponse(errBody []byte, statusCode int, operation string) error {
	// Read response body safely
	body, err := io.ReadAll(bytes.NewReader(errBody))
	if err != nil {
		// cli.logger.Printf("%s request failed with error %d (could not read response body, reason: %s)", operation, statusCode, err.Error())
		return fmt.Errorf("%s failed: error (status %d) (could not read response body, reason: %s)", operation, statusCode, err.Error())
	}
	// cli.logger.Printf("%s request failed with error %d: %s", operation, statusCode, string(body))
	return fmt.Errorf("%s failed: error (status %d): %s", operation, statusCode, string(body))
}

// OnboardAppPkg onboards a new application package.
//
// This method validates the request parameters and submits an onboarding request
// to the Northbound service. The service will process the package from the specified
// source and make it available for deployment.
//
// Parameters:
//   - params: The onboarding request parameters including name, source type, and source details
//
// Returns:
//   - *AppPkgOnboardingResp: The onboarding response with package details
//   - error: An error if validation fails or the request cannot be processed
//
// Example:
//
//	req := AppPkgOnboardingReq{
//	    Name: "my-app",
//	    SourceType: "git",
//	    Source: map[string]interface{}{"url": "https://github.com/user/app.git"},
//	}
//	resp, err := cli.OnboardAppPkg(req)
func (cli *NbiApiClient) OnboardAppPkg(params AppPkgOnboardingReq) (*AppPkgOnboardingResp, error) {
	// Validate required parameters
	if params.Metadata.Name == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}
	if params.Spec.SourceType == "" {
		return nil, fmt.Errorf("source type cannot be empty")
	}

	// Create client and context
	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	// Make API request
	resp, err := client.OnboardAppPackage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("onboard app package request failed: %s", err.Error())
	}
	defer resp.Body.Close()

	// Parse response
	pkgResp, err := nonStdWfmNbi.ParseOnboardAppPackageResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse onboard app package response: %s", err.Error())
	}

	// Handle response based on status code
	switch pkgResp.StatusCode() {
	case 200, 202:
		// cli.logger.Printf("Application onboard request accepted for package: %s", params.Metadata.Name)
		if pkgResp.JSON202 != nil {
			return pkgResp.JSON202, nil
		}
		return nil, nil
	default:
		return nil, cli.handleErrorResponse(pkgResp.Body, pkgResp.StatusCode(), "onboard app package")
	}
}

// GetAppPkg retrieves details for a specific application package.
//
// Parameters:
//   - pkgId: The unique identifier of the package to retrieve
//
// Returns:
//   - *AppPkgSummary: The package summary with details
//   - error: An error if the package is not found or cannot be retrieved
func (cli *NbiApiClient) GetAppPkg(pkgId string) (*AppPkgSummary, error) {
	if pkgId == "" {
		return nil, fmt.Errorf("package ID cannot be empty")
	}

	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.GetAppPackage(ctx, pkgId)
	if err != nil {
		return nil, fmt.Errorf("get app package request failed: %w", err)
	}
	defer resp.Body.Close()

	pkgResp, err := nonStdWfmNbi.ParseGetAppPackageResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse get app package response: %w", err)
	}

	switch pkgResp.StatusCode() {
	case 200:
		// cli.logger.Printf("Successfully retrieved package: %s", pkgId)
		return pkgResp.JSON200, nil
	default:
		return nil, cli.handleErrorResponse(pkgResp.Body, pkgResp.StatusCode(), "get app package")
	}
}

// ListAppPkgs retrieves a list of application packages.
//
// Parameters:
//   - params: Optional filtering and pagination parameters
//
// Returns:
//   - *ListAppPkgsResp: The list response containing packages and metadata
//   - error: An error if the request cannot be processed
func (cli *NbiApiClient) ListAppPkgs(params ListAppPkgsParams) (*ListAppPkgsResp, error) {
	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.ListAppPackages(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("list app packages request failed: %w", err)
	}
	defer resp.Body.Close()

	pkgResp, err := nonStdWfmNbi.ParseListAppPackagesResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list app packages response: %w", err)
	}

	switch pkgResp.StatusCode() {
	case 200:
		// packageCount := 0
		// if pkgResp.JSON200 != nil {
		// 	packageCount = len(pkgResp.JSON200.Items)
		// }
		// cli.logger.Printf("Successfully listed %d packages", packageCount)
		return pkgResp.JSON200, nil
	default:
		return nil, cli.handleErrorResponse(pkgResp.Body, pkgResp.StatusCode(), "list app packages")
	}
}

// DeleteAppPkg deletes a specific application package.
//
// Parameters:
//   - pkgId: The unique identifier of the package to delete
//
// Returns:
//   - error: An error if the package cannot be deleted
func (cli *NbiApiClient) DeleteAppPkg(pkgId string) error {
	if pkgId == "" {
		return fmt.Errorf("package ID cannot be empty")
	}

	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.DeleteAppPackage(ctx, pkgId, &nonStdWfmNbi.DeleteAppPackageParams{})
	if err != nil {
		return fmt.Errorf("delete app package request failed: %w", err)
	}
	defer resp.Body.Close()

	pkgResp, err := nonStdWfmNbi.ParseDeleteAppPackageResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to parse delete app package response: %w", err)
	}

	switch pkgResp.StatusCode() {
	case 200, 202:
		// cli.logger.Printf("Successfully deleted package: %s", pkgId)
		return nil
	default:
		return cli.handleErrorResponse(pkgResp.Body, pkgResp.StatusCode(), "delete app package")
	}
}

func (cli *NbiApiClient) CreateDeployment(params DeploymentReq) (*DeploymentResp, error) {
	// Validate required parameters
	// Create client and context
	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	// Make API request
	resp, err := client.CreateApplicationDeployment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("onboard app package request failed: %s", err.Error())
	}
	defer resp.Body.Close()

	// Parse response
	pkgResp, err := nonStdWfmNbi.ParseCreateApplicationDeploymentResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse onboard app package response: %s", err.Error())
	}

	// Handle response based on status code
	switch pkgResp.StatusCode() {
	case 200, 202:
		// cli.logger.Printf("Application deployment request accepted for package: %s", params.Spec.AppPackageRef.Id)
		if pkgResp.JSON202 != nil {
			return pkgResp.JSON202, nil
		}
		return nil, nil
	default:
		return nil, cli.handleErrorResponse(pkgResp.Body, pkgResp.StatusCode(), "create app deployment")
	}
}

// GetDeployment retrieves details for a specific application deployment.
func (cli *NbiApiClient) GetDeployment(deploymentId string) (*DeploymentResp, error) {
	if deploymentId == "" {
		return nil, fmt.Errorf("deployment ID cannot be empty")
	}

	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.GetApplicationDeployment(ctx, deploymentId)
	if err != nil {
		return nil, fmt.Errorf("get app package request failed: %w", err)
	}
	defer resp.Body.Close()

	deploymentResp, err := nonStdWfmNbi.ParseGetApplicationDeploymentResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse get app deployment response: %w", err)
	}

	switch deploymentResp.StatusCode() {
	case 200:
		// cli.logger.Printf("Successfully retrieved package: %s", deploymentId)
		return deploymentResp.JSON200, nil
	default:
		return nil, cli.handleErrorResponse(deploymentResp.Body, deploymentResp.StatusCode(), "get app deployment")
	}
}

// ListDeployments retrieves a list of application packages.
func (cli *NbiApiClient) ListDeployments(params DeploymentListParams) (*DeploymentListResp, error) {
	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.ListApplicationDeployments(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("list app packages request failed: %w", err)
	}
	defer resp.Body.Close()

	deploymentListResp, err := nonStdWfmNbi.ParseListApplicationDeploymentsResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list app deployment response: %w", err)
	}

	switch deploymentListResp.StatusCode() {
	case 200:
		// packageCount := 0
		// if deploymentListResp.JSON200 != nil {
		// 	packageCount = len(deploymentListResp.JSON200.Items)
		// }
		// cli.logger.Printf("Successfully listed %d deployments", packageCount)
		return deploymentListResp.JSON200, nil
	default:
		return nil, cli.handleErrorResponse(deploymentListResp.Body, deploymentListResp.StatusCode(), "list app deployments")
	}
}

func (cli *NbiApiClient) DeleteDeployment(deploymentId string) error {
	if deploymentId == "" {
		return fmt.Errorf("deployment ID cannot be empty")
	}

	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.DeleteApplicationDeployment(ctx, deploymentId)
	if err != nil {
		return fmt.Errorf("delete app deployment request failed: %w", err)
	}
	defer resp.Body.Close()

	deploymentResp, err := nonStdWfmNbi.ParseDeleteApplicationDeploymentResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to parse delete app deployment response: %w", err)
	}

	switch deploymentResp.StatusCode() {
	case 200, 202:
		// cli.logger.Printf("Successfully deleted deployment: %s", deploymentId)
		return nil
	default:
		return cli.handleErrorResponse(deploymentResp.Body, deploymentResp.StatusCode(), "delete app deployment")
	}
}

func (cli *NbiApiClient) ListDevices() (*DeviceListResp, error) {
	client, err := cli.createNonStdNbiClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := cli.createContext()
	defer cancel()

	resp, err := client.ListDevices(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list devices request failed: %w", err)
	}
	defer resp.Body.Close()

	deviceListResp, err := nonStdWfmNbi.ParseListDevicesResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list device response: %w", err)
	}

	switch deviceListResp.StatusCode() {
	case 200:
		// deviceCount := 0
		// if deviceListResp.JSON200 != nil {
		// 	deviceCount = len(deviceListResp.JSON200.Items)
		// }
		// cli.logger.Printf("Successfully listed %d devices", deviceCount)
		return deviceListResp.JSON200, nil
	default:
		return nil, cli.handleErrorResponse(deviceListResp.Body, deviceListResp.StatusCode(), "list devices")
	}
}
