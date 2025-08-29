package workloads

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/margo/dev-repo/shared-lib/file"
)

type DockerComposeClient struct {
	dockerClient *client.Client
	composeAPI   api.Service
	workingDir   string
}

type DockerConnectionViaHttp struct {
	Protocol   string
	Host       string
	Port       uint16
	CaCertPath string
	CertPath   string
	KeyPath    string
}

type DockerConnectionViaSocket struct {
	SocketPath string
}

type DockerConnectivityParams struct {
	ViaHttp   *DockerConnectionViaHttp
	ViaSocket *DockerConnectionViaSocket
}

// ComposeStatus represents the status of a Docker Compose deployment
type ComposeStatus struct {
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Services  []ServiceStatus `json:"services"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ServiceStatus struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Image       string   `json:"image"`
	Ports       []string `json:"ports"`
	ContainerID string   `json:"container_id"`
	Health      string   `json:"health"`
}

func NewDockerComposeClient(params DockerConnectivityParams, workingDir string) (*DockerComposeClient, error) {
	var dockerClient *client.Client
	var err error

	if workingDir == "" {
		return nil, fmt.Errorf("working directory path should be a valid path, existing value was: %s", workingDir)
	}

	if params.ViaSocket != nil {
		dockerClient, err = client.NewClientWithOpts(
			client.WithHost(params.ViaSocket.SocketPath),
			client.WithAPIVersionNegotiation(),
		)
	} else if params.ViaHttp != nil {
		hostURL := fmt.Sprintf("%s://%s:%d", params.ViaHttp.Protocol, params.ViaHttp.Host, params.ViaHttp.Port)
		dockerClient, err = client.NewClientWithOpts(
			client.WithHost(hostURL),
			client.WithTLSClientConfig(params.ViaHttp.CaCertPath, params.ViaHttp.CertPath, params.ViaHttp.KeyPath),
			client.WithAPIVersionNegotiation(),
		)
	} else {
		return nil, fmt.Errorf("no connection parameters provided")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	// Create Docker CLI instance
	cli, err := command.NewDockerCli(
		command.WithInputStream(os.Stdin),
		command.WithOutputStream(os.Stdout),
		command.WithErrorStream(os.Stderr),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker CLI: %w", err)
	}

	// Initialize CLI with client
	opts := &flags.ClientOptions{
		Debug: true,
	}
	if params.ViaSocket != nil {
		opts.Hosts = []string{params.ViaSocket.SocketPath}
	}

	if err := cli.Initialize(opts); err != nil {
		return nil, fmt.Errorf("failed to initialize docker CLI: %w", err)
	}

	// Create Compose API service with CLI
	composeAPI := compose.NewComposeService(cli)

	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	return &DockerComposeClient{
		dockerClient: dockerClient,
		composeAPI:   composeAPI,
		workingDir:   workingDir,
	}, nil
}

func (c *DockerComposeClient) DeployCompose(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Load compose project
	project, err := c.loadComposeProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return fmt.Errorf("failed to load compose project: %w", err)
	}

	fmt.Println("ProjectName", project.Name, "working directory", project.WorkingDir, "filename", project.Configs)

	err = c.composeAPI.Down(ctx, project.Name, api.DownOptions{
		RemoveOrphans: true,
		Project:       project,
		Volumes:       true,
		// Images:        "all",
	})
	if err != nil {
		fmt.Println("Failed to bring down the compose", "err", err.Error())
	}

	time.Sleep(time.Second * 10)

	if err := c.forceCleanupProject(ctx, projectName); err != nil {
		fmt.Println("failed to force cleanup the project", "error", err.Error())
	}

	// Load compose project
	project, err = c.loadComposeProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return fmt.Errorf("failed to load compose project: %w", err)
	}

	fmt.Println("ProjectName", project.Name, "working directory", project.WorkingDir, "filename", project.Configs)

	// Create containers first
	err = c.composeAPI.Create(ctx, project, api.CreateOptions{
		RemoveOrphans: true,
		Recreate:      "all",
	})
	if err != nil {
		time.Sleep(time.Second * 10)

		_ = c.composeAPI.Down(ctx, project.Name, api.DownOptions{
			RemoveOrphans: true,
			Volumes:       true,
			Project:       project,
		})
		return fmt.Errorf("failed to create containers: %w", err)
	}

	time.Sleep(time.Second * 10)
	// Load compose project
	project, err = c.loadComposeProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return fmt.Errorf("failed to load compose project: %w", err)
	}

	fmt.Println("ProjectName", project.Name, "working directory", project.WorkingDir, "filename", project.Configs)
	err = c.composeAPI.Start(ctx, project.Name, api.StartOptions{
		Project: project,
		Wait:    true,
	})
	if err != nil {
		time.Sleep(time.Second * 5)
		_ = c.composeAPI.Down(ctx, project.Name, api.DownOptions{
			RemoveOrphans: true,
			Project:       project,
		})
		return fmt.Errorf("failed to create containers: %w", err)
	}

	return nil
}

func (c *DockerComposeClient) DeployComposeFromURL(ctx context.Context, projectName string, composeFileURL string, envVars map[string]string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if strings.TrimSpace(composeFileURL) == "" {
		return fmt.Errorf("compose file URL cannot be empty")
	}

	// Fetch compose file content
	composeContent, err := c.FetchComposeFileFromURL(ctx, composeFileURL, projectName)
	if err != nil {
		return fmt.Errorf("failed to fetch compose file: %w", err)
	}

	return c.DeployCompose(ctx, projectName, composeContent, envVars)
}

func (c *DockerComposeClient) RemoveCompose(ctx context.Context, projectName string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	return c.composeAPI.Down(ctx, projectName, api.DownOptions{
		RemoveOrphans: true,
		Volumes:       true,
	})
}

func (c *DockerComposeClient) GetComposeStatus(ctx context.Context, composeFile string, projectName string) (*ComposeStatus, error) {
	if strings.TrimSpace(projectName) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	project, err := c.loadComposeProject(ctx, projectName, composeFile, nil)

	// Get project containers
	containers, err := c.composeAPI.Ps(ctx, projectName, api.PsOptions{
		All:     true,
		Project: project,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get project status: %w", err)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("compose project %s not found", projectName)
	}

	var services []ServiceStatus
	runningCount := 0
	createdCount := 0

	for _, container := range containers {
		status := "stopped"
		if strings.ToLower(container.State) == "running" {
			status = "running"
			runningCount++
		}

		if strings.ToLower(container.State) == "created" {
			status = "running"
			createdCount++
		}

		// Convert ports
		ports := make([]string, 0)
		for _, port := range container.Publishers {
			ports = append(ports, fmt.Sprintf("%d:%d", port.PublishedPort, port.TargetPort))
		}

		services = append(services, ServiceStatus{
			Name:        container.Service,
			Status:      status,
			Image:       container.Image,
			Ports:       ports,
			ContainerID: container.ID[:12],
			Health:      container.Health,
		})
	}

	// Determine overall status
	overallStatus := "stopped"
	if runningCount == len(services) {
		overallStatus = "running"
	} else if runningCount > 0 {
		overallStatus = "partial"
	}

	return &ComposeStatus{
		Name:      projectName,
		Status:    overallStatus,
		Services:  services,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (c *DockerComposeClient) RestartCompose(ctx context.Context, projectName string) error {
	return c.composeAPI.Restart(ctx, projectName, api.RestartOptions{})
}

func (c *DockerComposeClient) UpdateCompose(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	return c.DeployCompose(ctx, projectName, composeFile, envVars)
}

func (c *DockerComposeClient) ComposeExists(ctx context.Context, composeFile string, projectName string) (bool, error) {
	_, err := c.GetComposeStatus(ctx, composeFile, projectName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Helper function to load compose project
func (c *DockerComposeClient) loadComposeProject(ctx context.Context, projectName string, composeFile string, envVars map[string]string) (*types.Project, error) {
	// Prepare environment
	environment := make([]string, 0)
	for k, v := range envVars {
		environment = append(environment, k+"="+v)
	}
	opts, err := cli.NewProjectOptions(
		[]string{composeFile},
		cli.WithName(projectName),
		// cli.WithConsistency(true),
		cli.WithInterpolation(true),
		cli.WithWorkingDirectory(c.workingDir),
		cli.WithEnv(environment),
		// cli.WithNormalization(true),
	)
	if err != nil {
		return nil, err
	}

	return cli.ProjectFromOptions(ctx, opts)
}

// FetchComposeFileFromURL - simplified version using io.ReadAll
func (c *DockerComposeClient) FetchComposeFileFromURL(ctx context.Context, url string, filenameToUse string) (string, error) {
	// Create request with context
	downloadResult, err := file.DownloadFileUsingHttp("GET", url, nil, nil, nil, &file.DownloadOptions{
		OutputPath:     filepath.Join(c.workingDir, filenameToUse),
		CreateDirs:     true,
		OverwriteExist: true,
		ResumeDownload: false,
		ProgressCallback: func(downloaded, total int64) {
			fmt.Printf("\nTotal: %d, Downloaded: %d", total, downloaded)
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	return downloadResult.FilePath, nil
}

func (c *DockerComposeClient) forceCleanupProject(ctx context.Context, projectName string) error {
	// Get all containers (including stopped ones)
	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Find containers that match the project name pattern
	var containersToRemove []string
	for _, containerObj := range containers {
		for _, name := range containerObj.Names {
			// Remove leading slash from container name
			cleanName := strings.TrimPrefix(name, "/")

			// Check if container name starts with our project name
			if strings.HasPrefix(cleanName, projectName+"-") || strings.HasPrefix(cleanName, projectName+"_") {
				containersToRemove = append(containersToRemove, containerObj.ID)
				break
			}
		}
	}

	// Force remove all matching containers
	for _, containerID := range containersToRemove {
		// Stop first (ignore errors)
		timeout := 5
		c.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{
			Timeout: &timeout,
		})

		// Force remove
		if err := c.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
			RemoveLinks:   false,
		}); err != nil {
			fmt.Printf("Warning: failed to remove container %s: %v\n", containerID, err)
			// Continue with other containers
		}
	}

	return nil
}

func (c *DockerComposeClient) ExtractContent(composeFilename string) ([]byte, error) {
	fileHandler, err := os.Open(composeFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer fileHandler.Close()

	content, err := io.ReadAll(fileHandler)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("compose file is empty")
	}

	return content, nil
}

// Helper function to get compose content from package location
func (c *DockerComposeClient) DownloadCompose(ctx context.Context, packageLocation string, keyLocation *string, filenameToUse string) (string, error) {
	// This is a simplified implementation
	// 1. Download from URL if it's a remote location
	// 2. Read from file system if it's a local path
	if strings.HasPrefix(packageLocation, "http://") || strings.HasPrefix(packageLocation, "https://") {
		filename, err := c.FetchComposeFileFromURL(ctx, packageLocation, filenameToUse)
		if err != nil {
			return "", fmt.Errorf("failed to download the compose file from: %s, err: %s", packageLocation, err.Error())
		}

		return filename, nil
	}

	// For now, assume it's inline YAML content
	return packageLocation, nil
}
