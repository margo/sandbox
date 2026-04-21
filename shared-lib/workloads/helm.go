package workloads

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"errors"

	"github.com/margo/sandbox/shared-lib/http"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// HelmClient represents a Helm client with common settings
type HelmClient struct {
	settings       *cli.EnvSettings
	config         *action.Configuration
	registryClient *registry.Client
	kubeClient     kubernetes.Interface
}

// HelmError represents typed Helm errors
type HelmError struct {
	Type    string
	Message string
	Err     error
}

func (e *HelmError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *HelmError) Unwrap() error {
	return e.Err
}

// Error types
const (
	ErrorTypeNotFound     = "NotFound"
	ErrorTypeOther        = "Other"
	ErrorTypeInvalidInput = "InvalidInput"
	ErrorTypeRegistry     = "Registry"
	ErrorTypeChart        = "Chart"
	ErrorTypeRelease      = "Release"
)

// NewHelmClient creates a new Helm client
func NewHelmClient(kubeconfigPath string) (*HelmClient, error) {

	settings := cli.New()
	if kubeconfigPath != "" {
		settings.KubeConfig = kubeconfigPath
	}

	config := new(action.Configuration)

	// Initialize action configuration
	if err := config.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		os.Getenv("HELM_DRIVER"),
		log.Printf,
	); err != nil {
		return nil, fmt.Errorf("failed to initialize helm configuration: %w", err)
	}

	// Create registry client for OCI support
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	// Create Kubernetes client for namespace management
	kubeClient, err := createKubeClient(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &HelmClient{
		settings:       settings,
		config:         config,
		registryClient: registryClient,
		kubeClient:     kubeClient,
	}, nil
}

// createKubeClient creates a Kubernetes client
func createKubeClient(kubeconfigPath string) (kubernetes.Interface, error) {

	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

type HelmRepoAuth struct {
	CertAuth              *HelmRepoCertAuthentication
	BasicAuth             *HelmRepoBasicAuthentication
	InsecureSkipTLSverify *bool `json:"insecure_skip_tls_verify"`
}

type HelmRepoBasicAuthentication struct {
	Username string
	Password string
}

type HelmRepoCertAuthentication struct {
	CertFile           string `json:"certFile"`
	KeyFile            string `json:"keyFile"`
	CAFile             string `json:"caFile"`
	PassCredentialsAll bool   `json:"pass_credentials_all"`
}

// validateInput validates common input parameters
func validateInput(releaseName, chart string) error {
	if strings.TrimSpace(releaseName) == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "release name cannot be empty",
		}
	}
	if strings.TrimSpace(chart) == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "chart cannot be empty",
		}
	}
	return nil
}

// LoginRegistry authenticates with an OCI registry
func (c *HelmClient) LoginRegistry(registryUrl, username, password string) error {
	if registryUrl == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "registry cannot be empty",
		}
	}

	err := c.registryClient.Login(registryUrl, registry.LoginOptBasicAuth(username, password))
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: fmt.Sprintf("failed to login to registry %s", registryUrl),
			Err:     err,
		}
	}

	log.Printf("Successfully logged in to registry: %s", registryUrl)
	return nil
}

// LogoutRegistry logs out from an OCI registry
func (c *HelmClient) LogoutRegistry(registryURL string) error {
	if registryURL == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "registry URL cannot be empty",
		}
	}

	err := c.registryClient.Logout(registryURL)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: fmt.Sprintf("failed to logout from registry %s", registryURL),
			Err:     err,
		}
	}

	return nil
}

// AddRepository adds a Helm repository with persistence
func (c *HelmClient) AddRepository(name, url string, auth HelmRepoAuth) error {
	if name == "" || url == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "repository name and URL cannot be empty",
		}
	}

	repoEntry := repo.Entry{
		Name: name,
		URL:  url,
	}

	if auth.BasicAuth != nil {
		repoEntry.Username = auth.BasicAuth.Username
		repoEntry.Password = auth.BasicAuth.Password
	}

	if auth.InsecureSkipTLSverify != nil {
		repoEntry.InsecureSkipTLSverify = *auth.InsecureSkipTLSverify
	}

	if auth.CertAuth != nil {
		repoEntry.CAFile = auth.CertAuth.CAFile
		repoEntry.CertFile = auth.CertAuth.CertFile
		repoEntry.KeyFile = auth.CertAuth.KeyFile
		repoEntry.PassCredentialsAll = auth.CertAuth.PassCredentialsAll
	}

	repository, err := repo.NewChartRepository(&repoEntry, getter.All(c.settings))
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "failed to create chart repository",
			Err:     err,
		}
	}

	if _, err := repository.DownloadIndexFile(); err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "failed to download repository index",
			Err:     err,
		}
	}

	// Persist repository to file
	repoFile := c.settings.RepositoryConfig
	f, err := repo.LoadFile(repoFile)
	if err != nil {
		f = repo.NewFile()
	}

	// Update or add repository
	f.Update(&repoEntry)
	if err := f.WriteFile(repoFile, 0644); err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "failed to persist repository configuration",
			Err:     err,
		}
	}

	log.Printf("Successfully added repository: %s", name)
	return nil
}

// InstallChart installs a Helm chart with enhanced error handling
func (c *HelmClient) InstallChart(
	ctx context.Context,
	releaseName, chart, namespace, revision string,
	wait bool,
	values map[string]interface{},
) error {
	if err := validateInput(releaseName, chart); err != nil {
		return err
	}

	if namespace == "" {
		namespace = "default"
	}

	install := action.NewInstall(c.config)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.Version = revision
	install.Wait = wait
	install.Timeout = 10 * time.Minute

	// Check if it's an OCI reference
	if strings.HasPrefix(chart, "oci://") {
		return c.installChartFromOCI(ctx, install, chart, revision, values)
	}

	// Traditional chart installation
	chartPath, err := install.LocateChart(chart, c.settings)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to locate chart",
			Err:     err,
		}
	}

	chartReq, err := loader.Load(chartPath)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to load chart",
			Err:     err,
		}
	}

	_, err = install.RunWithContext(ctx, chartReq, values)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: "failed to install chart",
			Err:     err,
		}
	}

	log.Printf("Successfully installed chart: %s as release: %s", chart, releaseName)
	return nil
}

// installChartFromOCI installs a chart from OCI registry
func (c *HelmClient) installChartFromOCI(
	ctx context.Context,
	install *action.Install,
	chartRef, version string,
	values map[string]interface{},
) error {
	// Pull chart from OCI registry
	// extract port from
	port, err := http.ExtractPortFromURI(chartRef)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "invalid uri of the oci registry",
			Err:     err,
		}
	}

	// assuming that 80 port will be for plain http connections
	if port == 80 || port == 8080 || port == 8081 {
		registry.ClientOptPlainHTTP()(c.registryClient)
	}

	chartRef = fmt.Sprintf(
		"%s:%s",
		chartRef,
		version,
	) // "ghcr.io/nginxinc/charts/nginx-ingress:0.0.0-edge"
	result, err := c.registryClient.Pull(chartRef, registry.PullOptWithChart(true))
	if err != nil {
		fmt.Println("installChartFromOCI", "err", err.Error())
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "failed to pull OCI chart",
			Err:     err,
		}
	}

	// Load the chart
	chartReq, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to load OCI chart",
			Err:     err,
		}
	}

	_, err = install.RunWithContext(ctx, chartReq, values)
	if err != nil {
		fmt.Println("error", err.Error())
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: "failed to install OCI chart",
			Err:     errors.Join(err),
		}
	}

	return nil
}

// InstallChartWithDryRun performs a dry run installation
func (c *HelmClient) InstallChartWithDryRun(
	ctx context.Context,
	releaseName, chart, namespace, revision string,
	values map[string]interface{},
) (string, error) {
	if err := validateInput(releaseName, chart); err != nil {
		return "", err
	}

	if namespace == "" {
		namespace = "default"
	}

	install := action.NewInstall(c.config)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.Version = revision
	install.DryRun = true

	chartPath, err := install.LocateChart(chart, c.settings)
	if err != nil {
		return "", &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to locate chart",
			Err:     err,
		}
	}

	chartReq, err := loader.Load(chartPath)
	if err != nil {
		return "", &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to load chart",
			Err:     err,
		}
	}

	release, err := install.RunWithContext(ctx, chartReq, values)
	if err != nil {
		return "", &HelmError{
			Type:    ErrorTypeRelease,
			Message: "dry run failed",
			Err:     err,
		}
	}

	return release.Manifest, nil
}

// UninstallChart uninstalls a Helm release with enhanced error handling
func (c *HelmClient) UninstallChart(ctx context.Context, name, namespace string) error {
	if strings.TrimSpace(name) == "" {
		return &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "release name cannot be empty",
		}
	}

	uninstall := action.NewUninstall(c.config)
	uninstall.Timeout = 5 * time.Minute

	_, err := uninstall.Run(name)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: fmt.Sprintf("failed to uninstall release %s", name),
			Err:     err,
		}
	}

	log.Printf("Successfully uninstalled release: %s", name)
	return nil
}

// UpdateChart upgrades a Helm release with enhanced error handling
func (c *HelmClient) UpdateChart(
	ctx context.Context,
	name, chart, namespace string,
	values map[string]interface{},
) error {
	if err := validateInput(name, chart); err != nil {
		return err
	}

	if namespace == "" {
		namespace = "default"
	}

	upgrade := action.NewUpgrade(c.config)
	upgrade.Namespace = namespace
	upgrade.Timeout = 10 * time.Minute

	// Check if it's an OCI reference
	if strings.HasPrefix(chart, "oci://") {
		return c.updateChartFromOCI(ctx, upgrade, name, chart, values)
	}

	// Traditional chart upgrade
	chartPath, err := upgrade.LocateChart(chart, c.settings)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to locate chart",
			Err:     err,
		}
	}

	chartReq, err := loader.Load(chartPath)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to load chart",
			Err:     err,
		}
	}

	_, err = upgrade.RunWithContext(ctx, name, chartReq, values)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: fmt.Sprintf("failed to upgrade release %s", name),
			Err:     err,
		}
	}

	log.Printf("Successfully upgraded release: %s", name)
	return nil
}

// updateChartFromOCI upgrades a chart from OCI registry
func (c *HelmClient) updateChartFromOCI(
	ctx context.Context,
	upgrade *action.Upgrade,
	releaseName, chartRef string,
	values map[string]interface{},
) error {
	// Get the current release to determine the version if not specified
	status := action.NewStatus(c.config)
	currentRelease, err := status.Run(releaseName)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: fmt.Sprintf("failed to get current release %s", releaseName),
			Err:     err,
		}
	}

	// Extract version from chartRef or use current version
	var version string

	// Use current chart version or "latest"
	if currentRelease.Chart != nil && currentRelease.Chart.Metadata != nil {
		version = currentRelease.Chart.Metadata.Version
	} else {
		version = "latest"
	}
	chartRef = fmt.Sprintf("%s:%s", chartRef, version)

	// Pull chart from OCI registry
	result, err := c.registryClient.Pull(chartRef, registry.PullOptWithChart(true))
	if err != nil {
		fmt.Println(
			"failed to pull chart",
			err.Error(),
			"chartref",
			chartRef,
			"releaseName",
			releaseName,
			"values",
			values,
		)
		return &HelmError{
			Type:    ErrorTypeRegistry,
			Message: "failed to pull OCI chart for upgrade",
			Err:     err,
		}
	}

	// Load the chart
	chartReq, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeChart,
			Message: "failed to load OCI chart for upgrade",
			Err:     err,
		}
	}

	_, err = upgrade.RunWithContext(ctx, releaseName, chartReq, values)
	if err != nil {
		return &HelmError{
			Type:    ErrorTypeRelease,
			Message: fmt.Sprintf("failed to upgrade OCI chart for release %s", releaseName),
			Err:     err,
		}
	}

	log.Printf("Successfully upgraded OCI chart for release: %s", releaseName)
	return nil
}

// ReleaseStatus represents the status of a Helm release
type ReleaseStatus struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Status      release.Status         `json:"status"`
	Revision    int                    `json:"revision"`
	Updated     string                 `json:"updated"`
	Chart       string                 `json:"chart"`
	AppVersion  string                 `json:"app_version"`
	Description string                 `json:"description"`
	Notes       string                 `json:"notes"`
	Values      map[string]interface{} `json:"values"`
}

// GetReleaseStatus retrieves the status of a Helm release
func (c *HelmClient) GetReleaseStatus(
	ctx context.Context,
	releaseName, namespace string,
) (*ReleaseStatus, error) {
	if strings.TrimSpace(releaseName) == "" {
		return nil, &HelmError{
			Type:    ErrorTypeInvalidInput,
			Message: "release name cannot be empty",
		}
	}

	status := action.NewStatus(c.config)
	release, err := status.Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, &HelmError{
				Type:    ErrorTypeNotFound,
				Message: fmt.Sprintf("failed to get status for release %s", releaseName),
				Err:     err,
			}
		}
		return nil, &HelmError{
			Type:    ErrorTypeOther,
			Message: fmt.Sprintf("failed to get status for release %s", releaseName),
			Err:     err,
		}
	}

	if release == nil {
		return nil, &HelmError{
			Type:    ErrorTypeNotFound,
			Message: fmt.Sprintf("release %s not found", releaseName),
		}
	}

	releaseStatus := &ReleaseStatus{
		Name:        release.Name,
		Namespace:   release.Namespace,
		Status:      release.Info.Status,
		Revision:    release.Version,
		Description: release.Info.Description,
		Notes:       release.Info.Notes,
		Updated:     release.Info.LastDeployed.Format("2006-01-02 15:04:05"),
	}

	if release.Chart != nil && release.Chart.Metadata != nil {
		releaseStatus.Chart = fmt.Sprintf(
			"%s-%s",
			release.Chart.Metadata.Name,
			release.Chart.Metadata.Version,
		)
		releaseStatus.AppVersion = release.Chart.Metadata.AppVersion
	}

	if release.Config != nil {
		releaseStatus.Values = release.Config
	} else {
		releaseStatus.Values = make(map[string]interface{})
	}

	return releaseStatus, nil
}

// ListReleases lists all Helm releases with filtering options
func (c *HelmClient) ListReleases(ctx context.Context, namespace string) ([]*ReleaseStatus, error) {
	list := action.NewList(c.config)

	if namespace != "" {
		list.AllNamespaces = false
	} else {
		list.AllNamespaces = true
	}

	releases, err := list.Run()
	if err != nil {
		return nil, &HelmError{
			Type:    ErrorTypeRelease,
			Message: "failed to list releases",
			Err:     err,
		}
	}

	var releaseStatuses []*ReleaseStatus
	for _, release := range releases {
		status := &ReleaseStatus{
			Name:        release.Name,
			Namespace:   release.Namespace,
			Status:      release.Info.Status,
			Revision:    release.Version,
			Description: release.Info.Description,
			Updated:     release.Info.LastDeployed.Format("2006-01-02 15:04:05"),
		}

		if release.Chart != nil && release.Chart.Metadata != nil {
			status.Chart = fmt.Sprintf(
				"%s-%s",
				release.Chart.Metadata.Name,
				release.Chart.Metadata.Version,
			)
			status.AppVersion = release.Chart.Metadata.AppVersion
		}

		releaseStatuses = append(releaseStatuses, status)
	}

	return releaseStatuses, nil
}

// GetReleaseHistory gets the revision history for a release
func (c *HelmClient) GetReleaseHistory(
	ctx context.Context,
	releaseName, namespace string,
) ([]*ReleaseStatus, error) {
	history := action.NewHistory(c.config)

	releases, err := history.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get history for release %s: %w", releaseName, err)
	}

	var releaseHistory []*ReleaseStatus
	for _, release := range releases {
		status := &ReleaseStatus{
			Name:        release.Name,
			Namespace:   release.Namespace,
			Status:      release.Info.Status,
			Revision:    release.Version,
			Description: release.Info.Description,
		}

		status.Updated = release.Info.LastDeployed.Format("2006-01-02 15:04:05")

		if release.Chart != nil && release.Chart.Metadata != nil {
			status.Chart = fmt.Sprintf(
				"%s-%s",
				release.Chart.Metadata.Name,
				release.Chart.Metadata.Version,
			)
			status.AppVersion = release.Chart.Metadata.AppVersion
		}

		releaseHistory = append(releaseHistory, status)
	}

	return releaseHistory, nil
}

// ReleaseExists checks if a release exists
func (c *HelmClient) ReleaseExists(
	ctx context.Context,
	releaseName, namespace string,
) (bool, error) {
	_, err := c.GetReleaseStatus(ctx, releaseName, namespace)
	if err != nil {
		// If the error is "not found", return false without error
		if fmt.Sprintf(
			"%v",
			err,
		) == fmt.Sprintf(
			"failed to get status for release %s: release: not found",
			releaseName,
		) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
