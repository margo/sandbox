package operation

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/margo/sandbox/shared-lib/workloads"
	mobyclient "github.com/moby/moby/client"
	"go.uber.org/multierr"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// composeFileNames lists supported compose file names in order of preference.
var composeFileNames = []string{
	"docker-compose.yaml",
	"docker-compose.yml",
	"compose.yaml",
	"compose.yml",
}

type DockerConnectivityParams struct {
	ViaSocket *DockerSocketParams
}

type DockerSocketParams struct {
	SocketPath string
}

type DockerComposeClient struct {
	service    workloads.ComposeClient
	workingDir string
}

func NewDockerComposeClient(
	params DockerConnectivityParams,
	workingDir string,
) (*DockerComposeClient, error) {
	if strings.TrimSpace(workingDir) == "" {
		return nil, fmt.Errorf("working directory path should be valid, got %q", workingDir)
	}

	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker CLI: %w", err)
	}

	clientOptions := &flags.ClientOptions{
		LogLevel:   "info",
		TLSOptions: &tlsconfig.Options{},
	}

	if params.ViaSocket != nil {
		socketPath := strings.TrimSpace(params.ViaSocket.SocketPath)
		if socketPath == "" {
			return nil, fmt.Errorf("Docker socket path cannot be empty")
		}
		if !strings.HasPrefix(socketPath, "unix://") {
			socketPath = "unix://" + socketPath
		}
		clientOptions.Hosts = []string{socketPath}
	} else {
		return nil, fmt.Errorf("ViaSocket must be provided for Docker connectivity")
	}

	if err := dockerCLI.Initialize(clientOptions); err != nil {
		return nil, fmt.Errorf("failed to initialize docker CLI: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := dockerCLI.Client().Ping(ctx, mobyclient.PingOptions{}); err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	if err := os.MkdirAll(workingDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	// Create a new Compose service instance
	service, err := workloads.NewComposeService(dockerCLI)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose service: %w", err)
	}

	return &DockerComposeClient{
		service:    service,
		workingDir: workingDir,
	}, nil
}

func (c *DockerComposeClient) generateAbsProjectFilepath(projectName string) string {
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
func (c *DockerComposeClient) DownloadCompose(ctx context.Context, repository string, revision string, projectName string) (string, error) {
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

	// Find the pulled tar.gz/.tgz and extract compose file from it
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

			// Move all contents of the top-level directory to projectDir
			// so relative references (env_file, configs) resolve correctly
			// Works for both root-level and SUP-01 top-level-dir archives
			topLevelDir := filepath.Dir(composeFile)
			topLevelEntries, err := os.ReadDir(topLevelDir)
			if err != nil {
				return "", fmt.Errorf("failed to read extracted directory: %w", err)
			}
			for _, e := range topLevelEntries {
				src := filepath.Join(topLevelDir, e.Name())
				dst := filepath.Join(projectDir, e.Name())
				if err := os.Rename(src, dst); err != nil {
					// fallback for cross-device moves (e.g. /tmp → /data)
					if copyErr := c.copyFile(src, dst); copyErr != nil {
						return "", fmt.Errorf("failed to move %s: %w", e.Name(), copyErr)
					}
				}
			}

			finalPath := filepath.Join(projectDir, filepath.Base(composeFile))

			_ = os.Remove(archivePath)
			_ = os.RemoveAll(extractDir)

			// Verify compose file exists at final path before returning
			if _, err := os.Stat(finalPath); err != nil {
				return "", fmt.Errorf(
					"compose file missing after extraction at %s: %w",
					finalPath,
					err,
				)
			}

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

func (c *DockerComposeClient) extractAndFindCompose(archivePath, destDir string) (string, error) {
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

func (c *DockerComposeClient) copyFile(src, dst string) error {
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

func (d *DockerComposeClient) ComposeExists(ctx context.Context, composeFile string, projectName string, envVars map[string]string) (bool, error) {
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return false, nil
	}

	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return false, err
	}

	summary, err := d.service.Ps(ctx, projectName, workloads.PsOptions{
		Project: project,
		All:     false,
	})
	if err != nil {
		return false, err
	}

	if len(summary) == 0 {
		return false, nil
	}
	return true, nil
}

func (d *DockerComposeClient) DeployCompose(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return err
	}

	if err := d.downProject(ctx, projectName, project); err != nil {
		removeErr := d.removeProjectContainers(ctx, projectName)
		if removeErr != nil {
			return fmt.Errorf("compose down and fallback removal failed: %w",
				multierr.Combine(
					err,
					fmt.Errorf("fallback removal failed: %w", removeErr),
				),
			)
		}
	}

	if err := d.service.Pull(ctx, project, workloads.PullOptions{
		IgnoreFailures: true,
	}); err != nil {
		return fmt.Errorf("failed to pull compose images: %w", err)
	}

	if err := d.service.Up(ctx, project); err != nil {
		return fmt.Errorf("failed to start compose project %q: %w", projectName, err)
	}
	return nil
}

func (c *DockerComposeClient) UpdateCompose(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	return c.DeployCompose(ctx, projectName, composeFile, envVars)
}

func (d *DockerComposeClient) loadProject(
	ctx context.Context,
	projectName string,
	composeFile string,
	envVars map[string]string,
) (*workloads.Project, error) {
	if strings.TrimSpace(projectName) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if d.service == nil {
		return nil, fmt.Errorf("compose service is not initialized")
	}

	absComposeFile, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute compose file path: %w", err)
	}
	fileInfo, err := os.Stat(absComposeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to access compose file %q: %w", absComposeFile, err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("compose file path is a directory: %s", absComposeFile)
	}

	project, err := d.service.LoadProject(ctx, workloads.ProjectLoadOptions{
		ProjectName: projectName,
		ConfigPaths: []string{absComposeFile},
		WorkingDir:  filepath.Dir(absComposeFile),
		Environment: envVars,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project %q: %w", projectName, err)
	}
	return project, nil
}

func (d *DockerComposeClient) Up(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return fmt.Errorf("Up: %w", err)
	}
	if err := d.service.Up(ctx, project); err != nil {
		return fmt.Errorf("failed to start compose project %q: %w", projectName, err)
	}
	return nil
}

func (d *DockerComposeClient) Down(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return fmt.Errorf("Down: %w", err)
	}

	return d.downProject(ctx, projectName, project)
}

func (d *DockerComposeClient) downProject(ctx context.Context, projectName string, project *workloads.Project) error {
	return d.service.Down(ctx, projectName, workloads.DownOptions{
		RemoveOrphans: true,
		Project:       *project,
		Volumes:       true,
		Images:        "all",
	})
}

func (d *DockerComposeClient) removeProjectContainers(ctx context.Context, projectName string) error {
	return d.service.Remove(ctx, projectName, workloads.RemoveOptions{
		Stop:    true,
		Volumes: true,
		Force:   true,
	})
}

func (d *DockerComposeClient) Pull(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return err
	}

	err = d.service.Pull(ctx, project, workloads.PullOptions{
		IgnoreFailures: true,
	})
	if err != nil {
		return fmt.Errorf("failed to pull compose images: %w", err)
	}

	return nil
}

func (d *DockerComposeClient) Ps(ctx context.Context, projectName string, composeFile string, envVars map[string]string) ([]workloads.ContainerSummary, error) {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	summary, err := d.service.Ps(ctx, projectName, workloads.PsOptions{
		Project: project,
		All:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get compose status: %w", err)
	}

	return summary, nil

}

func (d *DockerComposeClient) RemoveCompose(ctx context.Context, projectName string) error {
	if strings.TrimSpace(projectName) == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	composeFile := d.generateAbsProjectFilepath(projectName)
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return d.removeProjectContainers(ctx, projectName)
	}

	project, err := d.loadProject(ctx, projectName, composeFile, nil)
	if err != nil {
		if removeErr := d.removeProjectContainers(ctx, projectName); removeErr != nil {
			return fmt.Errorf(
				"failed to load compose project and remove its containers: %w",
				multierr.Combine(err, removeErr),
			)
		}
	} else {
		err = d.service.Down(ctx, projectName, workloads.DownOptions{
			Project:       *project,
			RemoveOrphans: true,
			Images:        "all",
			Volumes:       true,
		})
		if err != nil {
			if removeErr := d.removeProjectContainers(ctx, projectName); removeErr != nil {
				return fmt.Errorf(
					"compose down and force removal failed: %w",
					multierr.Combine(err, removeErr),
				)
			}
		}
	}

	projectDir := filepath.Join(d.workingDir, projectName)
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("failed to remove project directory: %w", err)
	}

	return nil
}

func (d *DockerComposeClient) Restart(ctx context.Context, projectName string, composeFile string, envVars map[string]string) error {
	project, err := d.loadProject(ctx, projectName, composeFile, envVars)
	if err != nil {
		return err
	}

	err = d.service.Restart(ctx, projectName, workloads.RestartOptions{
		Project: project,
	})
	if err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return nil
}
