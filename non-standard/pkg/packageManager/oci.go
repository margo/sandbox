package packageManager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/margo/sandbox/non-standard/pkg/models"
)

// OCILoader loads packages from OCI registries using ORAS CLI
type OCILoader struct {
	config    *Config
	dirLoader *DirectoryLoader
}

func NewOCILoader(config *Config) *OCILoader {
	return &OCILoader{
		config:    config,
		dirLoader: NewDirectoryLoader(config),
	}
}

func (l *OCILoader) Type() SourceType {
	return SourceOCI
}

func (l *OCILoader) Load(
	ctx context.Context,
	source string,
	opts *LoadOptions,
) (string, *models.AppPkg, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp(l.config.TempDir, "margo-oci-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Parse registry and repository from source
	registry, repo := l.parseSource(source)

	// Login if credentials provided
	if opts.Username != "" && opts.Token != "" {
		if err := l.login(ctx, registry, opts); err != nil {
			_ = os.RemoveAll(tempDir)
			return "", nil, err
		}
	}

	// Pull artifact
	reference := fmt.Sprintf("%s/%s:%s", registry, repo, opts.Tag)
	if err := l.pull(ctx, reference, tempDir, opts); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}

	// Load using directory loader
	path, pkg, err := l.dirLoader.Load(ctx, tempDir, opts)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}

	return path, pkg, nil
}

func (l *OCILoader) parseSource(source string) (registry, repo string) {
	// Strip protocol
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimPrefix(source, "https://")

	// Split registry and repo
	parts := strings.SplitN(source, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return source, ""
}

func (l *OCILoader) login(
	ctx context.Context,
	registry string,
	opts *LoadOptions,
) error {
	args := []string{"login", registry, "-u", opts.Username, "-p", opts.Token}

	if strings.HasPrefix(registry, "http://") {
		args = append(args, "--plain-http")
	} else if opts.Insecure {
		args = append(args, "--insecure")
	}

	cmd := exec.CommandContext(ctx, "oras", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oras login failed: %w", err)
	}

	return nil
}

func (l *OCILoader) pull(
	ctx context.Context,
	reference, destDir string,
	opts *LoadOptions,
) error {
	args := []string{"pull", reference}

	if strings.Contains(reference, "http://") || opts.IsHTTP {
		args = append(args, "--plain-http")
	} else if opts.Insecure {
		args = append(args, "--insecure")
	}

	cmd := exec.CommandContext(ctx, "oras", args...)
	cmd.Dir = destDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"oras pull failed: %w, output: %s",
			err,
			string(output),
		)
	}

	return nil
}
