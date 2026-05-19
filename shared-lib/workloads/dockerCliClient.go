package workloads

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/margo/sandbox/shared-lib/file"
)

type DockerComposeCliClient struct {
	workingDir   string
	dockerBinary string
	params       DockerConnectivityParams
}

// CLI output structures for parsing
type ComposeContainer struct {
	ID         string      `json:"ID"`
	Name       string      `json:"Name"`
	Image      string      `json:"Image"`
	Command    string      `json:"Command"`
	Project    string      `json:"Project"`
	Service    string      `json:"Service"`
	State      string      `json:"State"`
	Health     string      `json:"Health"`
	ExitCode   int         `json:"ExitCode"`
	Publishers []Publisher `json:"Publishers"`
}

type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

func NewDockerComposeCliClient(
	params DockerConnectivityParams,
	workingDir string,
) (*DockerComposeCliClient, error) {
	if workingDir == "" {
		return nil, fmt.Errorf(
			"working directory path should be a valid path, existing value was: %s",
			workingDir,
		)
	}

	// Find docker binary
	dockerBinary, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker binary not found in PATH: %w", err)
	}

	// Test docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerBinary, "version")
	cmd.Env = prepareDockerEnv(params, nil)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	// Create working directory
	if err := os.MkdirAll(workingDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	return &DockerComposeCliClient{
		workingDir:   workingDir,
		dockerBinary: dockerBinary,
		params:       params,
	}, nil
}

func (c *DockerComposeCliClient) DeployCompose(
	ctx context.Context,
	projectName string,
	composeFile string,
	envVars map[string]string,
) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	fmt.Printf("Starting deployment for project: %s\n", projectName)
	fmt.Printf("Using compose file: %s\n", composeFile)

	// Ensure compose file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("compose file does not exist: %s", composeFile)
	}

	// Extract directory and filename separately
	projectDir := filepath.Dir(composeFile)
	composeFileName := filepath.Base(composeFile)

	fmt.Printf("Project directory: %s\n", projectDir)
	fmt.Printf("Compose filename: %s\n", composeFileName)

	// Step 1: Force cleanup of existing containers
	fmt.Printf("Cleaning up existing containers for project: %s\n", projectName)

	// First try compose down with force removal
	downCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"down", "--remove-orphans", "--volumes")

	downCmd.Dir = projectDir
	downCmd.Env = prepareDockerEnv(c.params, envVars)

	downOutput, err := downCmd.CombinedOutput()
	fmt.Printf("Down command output: %s\n", string(downOutput))
	if err != nil {
		fmt.Printf("Compose down failed: %v\n", err)

		// If compose down fails, try to remove containers manually
		if err := c.forceRemoveProjectContainers(ctx, projectName); err != nil {
			fmt.Printf("Manual container removal failed: %v\n", err)
		}
	}

	// Step 2: Pull latest images
	fmt.Printf("Pulling latest images for project: %s\n", projectName)
	pullCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"pull")

	pullCmd.Dir = projectDir
	pullCmd.Env = prepareDockerEnv(c.params, envVars)

	pullOutput, err := pullCmd.CombinedOutput()
	fmt.Printf("Pull command output: %s\n", string(pullOutput))
	if err != nil {
		fmt.Printf("Pull command failed (continuing anyway): %v\n", err)
	}

	// Step 3: Start containers
	fmt.Printf("Starting containers for project: %s\n", projectName)
	upCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"up", "-d", "--force-recreate")

	upCmd.Dir = projectDir
	upCmd.Env = prepareDockerEnv(c.params, envVars)

	upOutput, err := upCmd.CombinedOutput()
	fmt.Printf("Up command output: %s\n", string(upOutput))
	if err != nil {
		return fmt.Errorf("failed to start containers: %s", string(upOutput))
	}

	status, err := c.GetComposeStatus(ctx, composeFile, projectName)
	if err != nil {
		return fmt.Errorf("deployment verification failed: %w", err)
	}

	fmt.Printf(
		"Deployment successful. Status: %s, Services: %d\n",
		status.Status,
		len(status.Services),
	)
	return nil
}

func (c *DockerComposeCliClient) forceRemoveProjectContainers(
	ctx context.Context,
	projectName string,
) error {
	fmt.Printf("Force removing containers for project: %s\n", projectName)

	// Use both label filter AND name filter to catch all containers
	listCmd := exec.CommandContext(ctx, c.dockerBinary, "ps", "-a",
		"--filter", fmt.Sprintf("name=%s-", projectName),
		"--format", "{{.ID}} {{.Names}}")

	listCmd.Env = prepareDockerEnv(c.params, nil)

	output, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Printf("No containers found for project: %s\n", projectName)
		return nil
	}

	// Force remove each container
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		containerID := parts[0]
		containerName := parts[1]

		fmt.Printf("Force removing container: %s (%s)\n", containerName, containerID)

		// Stop and remove container
		removeCmd := exec.CommandContext(ctx, c.dockerBinary, "rm", "-f", containerID)
		removeCmd.Env = prepareDockerEnv(c.params, nil)

		if removeOutput, err := removeCmd.CombinedOutput(); err != nil {
			fmt.Printf(
				"Failed to remove container %s: %v, output: %s\n",
				containerName,
				err,
				string(removeOutput),
			)
		} else {
			fmt.Printf("Successfully removed container: %s\n", containerName)
		}
	}

	return nil
}

func (c *DockerComposeCliClient) DeployComposeFromURL(
	ctx context.Context,
	projectName string,
	composeFileURL string,
	envVars map[string]string,
) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if strings.TrimSpace(composeFileURL) == "" {
		return fmt.Errorf("compose file URL cannot be empty")
	}

	// Fetch compose file content
	composeFile, err := c.fetchComposeFileFromURL(ctx, composeFileURL, projectName)
	if err != nil {
		return fmt.Errorf("failed to fetch compose file: %w", err)
	}

	return c.DeployCompose(ctx, projectName, composeFile, envVars)
}

func (c *DockerComposeCliClient) RemoveCompose(ctx context.Context, projectName string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Find compose file for this project
	composeFile := c.generateAbsProjectFilepath(projectName)
	fmt.Printf("Attempting to remove compose project: %s\n", projectName)
	fmt.Printf("Looking for compose file at: %s\n", composeFile)

	// Check if compose file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Printf("Compose file not found, trying manual container removal\n")
		return c.forceRemoveProjectContainers(ctx, projectName)
	}

	cmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", filepath.Base(composeFile), // Use ONLY the filename
		"-p", projectName,
		"down", "--remove-orphans", "--volumes", "--rmi", "local")

	cmd.Dir = filepath.Dir(composeFile) // Set working directory
	cmd.Env = prepareDockerEnv(c.params, nil)

	output, err := cmd.CombinedOutput()
	fmt.Printf("Remove command output: %s\n", string(output))

	if err != nil {
		fmt.Printf("Compose down failed, trying manual removal: %v\n", err)
		if err := c.forceRemoveProjectContainers(ctx, projectName); err != nil {
			return fmt.Errorf("manual removal also failed: %w", err)
		}
	}

	// Verify containers are actually removed
	if err := c.verifyContainersRemoved(ctx, projectName); err != nil {
		// Try one more time with force removal if verification fails
		fmt.Printf("Verification failed, attempting final cleanup: %v\n", err)
		if finalErr := c.forceRemoveProjectContainers(ctx, projectName); finalErr != nil {

			return fmt.Errorf("containers still running after all removal attempts: %w", err)
		}
	}

	// Clean up project directory
	projectDir := filepath.Join(c.workingDir, projectName)
	return os.RemoveAll(projectDir)
}

func (c *DockerComposeCliClient) GetComposeStatus(
	ctx context.Context,
	composeFile string,
	projectName string,
) (*ComposeStatus, error) {
	if strings.TrimSpace(projectName) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	// Verify compose file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("compose file does not exist: %s", composeFile)
	}

	fmt.Printf("[DEBUG] composeFile: %s\n", composeFile)
	fmt.Printf("[DEBUG] projectName: %s\n", projectName)
	fmt.Printf("[DEBUG] dockerBinary: %s\n", c.dockerBinary)

	// Use absolute path for compose file
	absComposeFile, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", filepath.Base(absComposeFile), // Use just filename
		"-p", projectName,
		"ps", "--format", "json", "--all")

	cmd.Dir = filepath.Dir(absComposeFile) // Set working directory
	cmd.Env = prepareDockerEnv(c.params, nil)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get compose status: %w, output: %s", err, string(output))
	}

	fmt.Printf("[DEBUG] Raw docker compose ps output: %s\n", string(output))

	// Handle empty output (no containers)
	if len(strings.TrimSpace(string(output))) == 0 {
		return &ComposeStatus{
			Name:      projectName,
			Status:    "stopped",
			Services:  []ServiceStatus{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}
	// Parse JSON output - it's a single JSON array, not line-by-line objects
	var containers []ComposeContainer

	// Try parsing as JSON array first
	if err := json.Unmarshal(output, &containers); err != nil {
		// If array parsing fails, try parsing line-by-line JSON objects
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		containers = make([]ComposeContainer, 0, len(lines))

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var container ComposeContainer
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				fmt.Printf("[DEBUG] Failed to parse line as JSON: %s, error: %v\n", line, err)
				continue
			}
			containers = append(containers, container)
		}

		// If still no containers parsed, return error
		if len(containers) == 0 {
			return nil, fmt.Errorf(
				"failed to parse any container JSON from output: %s",
				string(output),
			)
		}
	}

	// If no containers found, return empty status
	if len(containers) == 0 {
		return &ComposeStatus{
			Name:      projectName,
			Status:    "stopped",
			Services:  []ServiceStatus{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	var services []ServiceStatus
	runningCount := 0

	for _, container := range containers {
		status := "stopped"
		if strings.Contains(strings.ToLower(container.State), "running") {
			status = "running"
			runningCount++
		} else if strings.Contains(strings.ToLower(container.State), "up") {
			status = "running"
			runningCount++
		}

		// Parse ports from Publishers array
		ports := []string{}
		for _, publisher := range container.Publishers {
			if publisher.PublishedPort > 0 {
				ports = append(
					ports,
					fmt.Sprintf("%d:%d", publisher.PublishedPort, publisher.TargetPort),
				)
			}
		}

		services = append(services, ServiceStatus{
			Name:        container.Service,
			Status:      status,
			Image:       container.Image,
			Ports:       ports,
			ContainerID: container.ID,
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

func (c *DockerComposeCliClient) RestartCompose(ctx context.Context, projectName string) error {
	composeFile := c.generateAbsProjectFilepath(projectName)

	cmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", filepath.Base(composeFile), // Use only filename
		"-p", projectName,
		"restart")

	cmd.Dir = filepath.Dir(composeFile) // Set working directory
	cmd.Env = prepareDockerEnv(c.params, nil)

	output, err := cmd.CombinedOutput()
	fmt.Printf("Restart command output: %s\n", string(output))

	if err != nil {

		return fmt.Errorf("failed to restart compose project: %s", string(output))

	}

	return nil
}

func (c *DockerComposeCliClient) verifyContainersRemoved(
	ctx context.Context,
	projectName string,
) error {
	// Check if any containers with this project name still exist
	listCmd := exec.CommandContext(ctx, c.dockerBinary, "ps", "-a",
		"--filter", fmt.Sprintf("name=%s-", projectName),
		"--format", "{{.Names}}")

	listCmd.Env = prepareDockerEnv(c.params, nil)

	output, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to verify removal: %w", err)
	}

	remainingContainers := strings.TrimSpace(string(output))
	if remainingContainers != "" {
		return fmt.Errorf("containers still exist: %s", remainingContainers)
	}

	return nil
}

func (c *DockerComposeCliClient) UpdateCompose(
	ctx context.Context,
	projectName string,
	composeFile string,
	envVars map[string]string,
) error {
	return c.DeployCompose(ctx, projectName, composeFile, envVars)
}

func (c *DockerComposeCliClient) ComposeExists(
	ctx context.Context,
	composeFile string,
	projectName string,
) (bool, error) {

	// First check if compose file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return false, nil
	}
	_, err := c.GetComposeStatus(ctx, composeFile, projectName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Helper function to prepare Docker environment variables
func prepareDockerEnv(params DockerConnectivityParams, envVars map[string]string) []string {
	env := os.Environ()

	// Set Docker host
	if params.ViaSocket != nil {

		env = append(env, fmt.Sprintf("DOCKER_HOST=unix://%s", params.ViaSocket.SocketPath))

	} else if params.ViaHttp != nil {
		hostURL := fmt.Sprintf(
			"%s://%s:%d",
			params.ViaHttp.Protocol,
			params.ViaHttp.Host,
			params.ViaHttp.Port,
		)
		env = append(env, fmt.Sprintf("DOCKER_HOST=%s", hostURL))

		if params.ViaHttp.CaCertPath != "" {
			env = append(
				env,
				fmt.Sprintf("DOCKER_CERT_PATH=%s", filepath.Dir(params.ViaHttp.CaCertPath)),
			)
			env = append(env, "DOCKER_TLS_VERIFY=1")
		}
	}

	// Add custom environment variables
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// composeFileNames lists supported compose file names in order of preference.
var composeFileNames = []string{
	"docker-compose.yaml",
	"docker-compose.yml",
	"compose.yaml",
	"compose.yml",
}

func (c *DockerComposeCliClient) generateAbsProjectFilepath(projectName string) string {
	projectDir := filepath.Join(c.workingDir, projectName)

	for _, name := range composeFileNames {
		path := filepath.Join(projectDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default fallback when file doesn't exist yet
	return filepath.Join(projectDir, composeFileNames[0])
}

// fetchComposeFileFromURL - simplified version using io.ReadAll
func (c *DockerComposeCliClient) fetchComposeFileFromURL(
	_ context.Context,
	url string,
	projectName string,
) (string, error) {
	// Create request with context
	downloadResult, err := file.DownloadFileUsingHttp(
		"GET",
		url,
		nil,
		nil,
		nil,
		&file.DownloadOptions{
			Timeout:        time.Second * 10,
			OutputPath:     c.generateAbsProjectFilepath(projectName),
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

func (c *DockerComposeCliClient) DownloadCompose(
	ctx context.Context,
	packageLocation string,
	keyLocation *string,
	projectName string,
) (string, error) {
	// Handle remote URLs

	isHTTP := strings.HasPrefix(packageLocation, "http://") ||
		strings.HasPrefix(packageLocation, "https://")
	if isHTTP {
		// Check if it's a tar.gz archive
		isTarGz := strings.HasSuffix(packageLocation, ".tar.gz") ||
			strings.HasSuffix(packageLocation, ".tgz")
		if isTarGz {
			return c.downloadAndExtractTarGz(
				ctx,
				packageLocation,
				keyLocation,
				projectName,
			)
		}
		// Direct compose file download
		return c.fetchComposeFileFromURL(ctx, packageLocation, projectName)
	}

	// Assume it's inline YAML content or local path
	return packageLocation, nil
}

func (c *DockerComposeCliClient) downloadAndExtractTarGz(
	ctx context.Context,
	archiveURL string,
	keyLocation *string,
	projectName string,
) (string, error) {
	projectDir := filepath.Join(c.workingDir, projectName)
	archivePath := filepath.Join(projectDir, ".archive.tar.gz")
	extractDir := filepath.Join(projectDir, ".extracted")

	if err := os.MkdirAll(projectDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create project directory: %w", err)
	}

	if _, err := file.DownloadFileUsingHttp(
		"GET",
		archiveURL,
		nil, nil, nil,
		&file.DownloadOptions{
			Timeout:        30 * time.Second,
			OutputPath:     archivePath,
			CreateDirs:     false,
			OverwriteExist: true,
			ResumeDownload: false,
		},
	); err != nil {
		return "", fmt.Errorf("failed to download archive: %w", err)
	}

	composeFile, err := c.extractAndFindCompose(archivePath, extractDir)
	if err != nil {
		return "", err
	}

	// Preserve original filename (compose.yaml, docker-compose.yml, etc.)
	finalComposePath := filepath.Join(projectDir, filepath.Base(composeFile))

	if err := os.Rename(composeFile, finalComposePath); err != nil {

		if copyErr := c.copyFile(composeFile, finalComposePath); copyErr != nil {
			return "", fmt.Errorf("failed to move compose file: %w", copyErr)
		}
	}

	return finalComposePath, nil
}
func (c *DockerComposeCliClient) extractAndFindCompose(
	archivePath, destDir string,
) (string, error) {

	f, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("invalid gzip format: %w", err)
	}
	// errcheck: handle gzr.Close error
	defer func() {
		if err := gzr.Close(); err != nil {
			fmt.Printf("Warning: failed to close gzip reader: %v\n", err)
		}
	}()

	tr := tar.NewReader(gzr)

	composeNames := make(map[string]bool, len(composeFileNames))
	for _, name := range composeFileNames {
		composeNames[name] = true

	}

	var foundCompose string

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		headerName := filepath.Clean(header.Name)
		if strings.Contains(headerName, "..") {
			return "", fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		target := filepath.Join(destDir, headerName)

		if !strings.HasPrefix(
			filepath.Clean(target)+string(os.PathSeparator),
			cleanDest,
		) {
			return "", fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0750); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return "", fmt.Errorf(
					"failed to create parent directory: %w",
					err,
				)
			}

			// Use fixed permission instead of header.Mode for security
			outFile, err := os.OpenFile( //nolint:gosec // path is sanitized above
				target,
				os.O_CREATE|os.O_RDWR|os.O_TRUNC,
				0600,
			)
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}

			_, copyErr := io.Copy(outFile, io.LimitReader(tr, 100*1024*1024))
			outFile.Close()

			if copyErr != nil {
				return "", fmt.Errorf("failed to write file: %w", copyErr)
			}

			if composeNames[filepath.Base(target)] && foundCompose == "" {
				foundCompose = target
			}

		}
	}

	if foundCompose == "" {
		return "", fmt.Errorf(
			"no compose file found in archive (expected: docker-compose.yaml, compose.yaml, etc.)",
		)
	}

	return foundCompose, nil
}

func (c *DockerComposeCliClient) copyFile(src, dst string) error {
	source, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}

	return dest.Sync()
}
