package workloads

import (
	"context"
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// HelmClient represents a Helm client with common settings
type HelmClient struct {
	settings *cli.EnvSettings
	config   *action.Configuration
	ctx      context.Context
}

// NewHelmClient creates a new Helm client
func NewHelmClient() (*HelmClient, error) {
	settings := cli.New()
	config := new(action.Configuration)
	ctx := context.Background()

	// Initialize action configuration
	if err := config.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, fmt.Errorf("failed to initialize helm configuration: %w", err)
	}

	return &HelmClient{
		settings: settings,
		config:   config,
		ctx:      ctx,
	}, nil
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

// AddRepository adds a Helm repository
func (c *HelmClient) AddRepository(name, url string, auth HelmRepoAuth) error {
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
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	if _, err := repository.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index: %w", err)
	}

	return nil
}

// InstallChart installs a Helm chart
func (c *HelmClient) InstallChart(releaseName, chart, namespace, revision string, wait bool, values map[string]interface{}) error {
	install := action.NewInstall(c.config)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.Version = revision
	install.Wait = wait

	chartPath, err := install.ChartPathOptions.LocateChart(chart, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	chartReq, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	_, err = install.RunWithContext(c.ctx, chartReq, values)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	return nil
}

// UninstallChart uninstalls a Helm release
func (c *HelmClient) UninstallChart(name, namespace string) error {
	uninstall := action.NewUninstall(c.config)

	_, err := uninstall.Run(name)
	if err != nil {
		return fmt.Errorf("failed to uninstall release %s: %w", name, err)
	}

	return nil
}

// UpdateChart upgrades a Helm release
func (c *HelmClient) UpdateChart(name, chart, namespace string, values map[string]interface{}) error {
	upgrade := action.NewUpgrade(c.config)
	upgrade.Namespace = namespace

	chartPath, err := upgrade.ChartPathOptions.LocateChart(chart, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	chartReq, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	_, err = upgrade.RunWithContext(c.ctx, name, chartReq, values)
	if err != nil {
		return fmt.Errorf("failed to upgrade release %s: %w", name, err)
	}

	return nil
}
