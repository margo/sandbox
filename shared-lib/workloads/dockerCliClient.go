package workloads

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/multierr"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
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

// DockerConnectionViaHttp defines HTTP connection parameters for Docker daemon.
type DockerConnectionViaHttp struct {
	Protocol   string
	Host       string
	Port       uint16
	CaCertPath string
	CertPath   string
	KeyPath    string
}

// DockerConnectionViaSocket defines Unix socket connection parameters for Docker daemon.
type DockerConnectionViaSocket struct {
	SocketPath string
}

// DockerConnectivityParams defines how to connect to the Docker daemon.
type DockerConnectivityParams struct {
	ViaHttp   *DockerConnectionViaHttp
	ViaSocket *DockerConnectionViaSocket
}

// ComposeStatus represents the status of a Docker Compose deployment.
type ComposeStatus struct {
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Services  []ServiceStatus `json:"services"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ServiceStatus represents the status of a single service in a Compose deployment.
type ServiceStatus struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Image       string   `json:"image"`
	Ports       []string `json:"ports"`
	ContainerID string   `json:"container_id"`
	Health      string   `json:"health"`
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
	if err := os.MkdirAll(workingDir, 0o750); err != nil {
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

	// Ensure compose file exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("compose file does not exist: %s", composeFile)
	}

	// Extract directory and filename separately
	projectDir := filepath.Dir(composeFile)
	composeFileName := filepath.Base(composeFile)

	downCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"down", "--remove-orphans", "--volumes")

	downCmd.Dir = projectDir
	downCmd.Env = prepareDockerEnv(c.params, envVars)

	if _, err := downCmd.CombinedOutput(); err != nil {
		_ = c.forceRemoveProjectContainers(ctx, projectName)
	}

	pullCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"pull")

	pullCmd.Dir = projectDir
	pullCmd.Env = prepareDockerEnv(c.params, envVars)

	// Discard pull output — warnings on stderr are expected and must not affect result
	pullCmd.Stdout = io.Discard
	pullCmd.Stderr = io.Discard
	_ = pullCmd.Run()

	upCmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", composeFileName,
		"-p", projectName,
		"up", "-d", "--force-recreate")

	upCmd.Dir = projectDir
	upCmd.Env = prepareDockerEnv(c.params, envVars)

	// Separate stdout and stderr — warnings must not pollute error message
	var upStderr strings.Builder
	upCmd.Stdout = io.Discard
	upCmd.Stderr = &upStderr

	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("failed to start containers: %s", upStderr.String())
	}

	if _, err := c.GetComposeStatus(ctx, composeFile, projectName); err != nil {
		return fmt.Errorf("deployment verification failed: %w", err)
	}

	return nil
}

func (c *DockerComposeCliClient) forceRemoveProjectContainers(
	ctx context.Context,
	projectName string,
) error {
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
		return nil
	}

	var errs error
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

		removeCmd := exec.CommandContext(ctx, c.dockerBinary, "rm", "-f", containerID)
		removeCmd.Env = prepareDockerEnv(c.params, nil)

		if removeOutput, err := removeCmd.CombinedOutput(); err != nil {
			errs = multierr.Append(
				errs,
				fmt.Errorf(
					"failed to remove container %s: %w, output: %s",
					containerName,
					err,
					string(removeOutput),
				),
			)
		}
	}

	return errs
}

func (c *DockerComposeCliClient) RemoveCompose(ctx context.Context, projectName string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	composeFile := c.generateAbsProjectFilepath(projectName)

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return c.forceRemoveProjectContainers(ctx, projectName)
	}

	cmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", filepath.Base(composeFile),
		"-p", projectName,
		"down", "--remove-orphans", "--volumes", "--rmi", "local")

	cmd.Dir = filepath.Dir(composeFile)
	cmd.Env = prepareDockerEnv(c.params, nil)

	var errs error
	if _, err := cmd.CombinedOutput(); err != nil {
		errs = multierr.Append(
			errs,
			fmt.Errorf("compose down failed: %w", err),
		)
		if forceErr := c.forceRemoveProjectContainers(ctx, projectName); forceErr != nil {
			errs = multierr.Append(
				errs,
				fmt.Errorf("manual removal also failed: %w", forceErr),
			)
			return errs
		}
	}

	if err := c.verifyContainersRemoved(ctx, projectName); err != nil {
		if forceErr := c.forceRemoveProjectContainers(ctx, projectName); forceErr != nil {
			return multierr.Append(
				errs,
				fmt.Errorf(
					"containers still running after all removal attempts: %w",
					multierr.Combine(err, forceErr),
				),
			)
		}
	}

	projectDir := filepath.Join(c.workingDir, projectName)
	if err := os.RemoveAll(projectDir); err != nil {
		errs = multierr.Append(
			errs,
			fmt.Errorf("failed to remove project directory: %w", err),
		)
	}

	return errs
}

func (c *DockerComposeCliClient) GetComposeStatus(
	ctx context.Context,
	composeFile string,
	projectName string,
) (*ComposeStatus, error) {
	if strings.TrimSpace(projectName) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("compose file does not exist: %s", composeFile)
	}

	absComposeFile, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.dockerBinary, "compose",
		"-f", filepath.Base(absComposeFile),
		"-p", projectName,
		"ps", "--format", "json", "--all")

	cmd.Dir = filepath.Dir(absComposeFile)
	cmd.Env = prepareDockerEnv(c.params, nil)

	// Separate stdout and stderr — stderr warnings must NOT pollute JSON stdout
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"failed to get compose status: %w, stderr: %s",
			err,
			stderr.String(),
		)
	}

	output := stdout.String()

	// Handle empty output (no containers) — correct fast path, unaffected by stderr warnings
	if len(strings.TrimSpace(output)) == 0 {
		return &ComposeStatus{
			Name:      projectName,
			Status:    "stopped",
			Services:  []ServiceStatus{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	// Parse JSON output - try array first, then line-by-line
	var containers []ComposeContainer

	if err := json.Unmarshal([]byte(output), &containers); err != nil {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		containers = make([]ComposeContainer, 0, len(lines))

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var container ComposeContainer
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				continue
			}
			containers = append(containers, container)
		}

		if len(containers) == 0 {
			return nil, fmt.Errorf(
				"failed to parse any container JSON from output: %s",
				output,
			)
		}
	}

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
		if strings.Contains(strings.ToLower(container.State), "running") ||
			strings.Contains(strings.ToLower(container.State), "up") {
			status = "running"
			runningCount++
		}

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
		"-f", filepath.Base(composeFile),
		"-p", projectName,
		"restart")

	cmd.Dir = filepath.Dir(composeFile)
	cmd.Env = prepareDockerEnv(c.params, nil)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to restart compose project: %s",
			string(output),
		)
	}

	return nil
}

func (c *DockerComposeCliClient) verifyContainersRemoved(
	ctx context.Context,
	projectName string,
) error {
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

// prepareDockerEnv builds the environment variable slice for docker commands.
func prepareDockerEnv(params DockerConnectivityParams, envVars map[string]string) []string {
	env := os.Environ()

	if params.ViaSocket != nil {
		env = append(
			env,
			fmt.Sprintf("DOCKER_HOST=unix://%s", params.ViaSocket.SocketPath),
		)
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
				fmt.Sprintf(
					"DOCKER_CERT_PATH=%s",
					filepath.Dir(params.ViaHttp.CaCertPath),
				),
			)
			env = append(env, "DOCKER_TLS_VERIFY=1")
		}
	}

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

// DownloadCompose pulls a compose archive from OCI registry using oras-go client,
// extracts it, and returns the path to the compose file.
// repository: oci://harbor.machine:8443/library/nextcloud-compose-archive
// revision:   1.0.0
func (c *DockerComposeCliClient) DownloadCompose(
	ctx context.Context,
	repository string,
	revision string,
	projectName string,
) (string, error) {
	if strings.TrimSpace(repository) == "" {
		return "", fmt.Errorf("repository cannot be empty")
	}
	if strings.TrimSpace(revision) == "" {
		return "", fmt.Errorf("revision cannot be empty")
	}

	projectDir := filepath.Join(c.workingDir, projectName)
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create project directory: %w", err)
	}

	// Strip oci:// prefix — oras-go expects host/repo format
	rawRef := strings.TrimPrefix(repository, "oci://")

	// Create a file store to download artifacts into projectDir
	store, err := file.New(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to create oras file store: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	// Create remote repository reference
	repo, err := remote.NewRepository(rawRef)
	if err != nil {
		return "", fmt.Errorf("failed to create remote repository for %s: %w", rawRef, err)
	}

	// Configure auth client with insecure TLS for self-signed certs (PoC/dev environment)
	repo.Client = &auth.Client{
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // PoC environment with self-signed certs
				},
			},
		},
		Cache: auth.NewCache(),
	}

	// Harbor on 8443 uses HTTPS — do NOT set PlainHTTP
	repo.PlainHTTP = false

	// Pull the artifact at the given tag/revision into the file store
	_, err = oras.Copy(ctx, repo, revision, store, revision, oras.DefaultCopyOptions)
	if err != nil {
		return "", fmt.Errorf("failed to pull compose archive from OCI %s:%s: %w",
			rawRef, revision, err)
	}

	// Find the pulled tar.gz and extract compose.yaml from it
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to read project directory after OCI pull: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
			archivePath := filepath.Join(projectDir, name)
			extractDir := filepath.Join(projectDir, ".extracted")

			composeFile, err := c.extractAndFindCompose(archivePath, extractDir)
			if err != nil {
				return "", fmt.Errorf("failed to extract compose file from archive: %w", err)
			}

			finalPath := filepath.Join(projectDir, filepath.Base(composeFile))
			if err := os.Rename(composeFile, finalPath); err != nil {
				if copyErr := c.copyFile(composeFile, finalPath); copyErr != nil {
					return "", fmt.Errorf("failed to move compose file: %w",
						multierr.Combine(err, copyErr))
				}
			}

			_ = os.Remove(archivePath)
			_ = os.RemoveAll(extractDir)

			return finalPath, nil
		}
	}

	// If no tar.gz found, check if compose file was pulled directly
	for _, name := range composeFileNames {
		candidate := filepath.Join(projectDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no compose file or tar.gz archive found after OCI pull from %s:%s",
		rawRef, revision)
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
	defer func() {
		_ = gzr.Close()
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
			if err := os.MkdirAll(target, 0o750); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return "", fmt.Errorf(
					"failed to create parent directory: %w",
					err,
				)
			}

			outFile, err := os.OpenFile( //nolint:gosec // path is sanitized above
				target,
				os.O_CREATE|os.O_RDWR|os.O_TRUNC,
				0o600,
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

	//nolint:gosec // path is sanitized above
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
