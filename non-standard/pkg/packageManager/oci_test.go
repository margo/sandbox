package packageManager

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pushPackageToRegistry pushes a package using ORAS CLI (matches production)
func pushPackageToRegistry(
	t *testing.T,
	pkgDir, registryURL, repo, tag string,
) error {
	// Check if oras is available
	if _, err := exec.LookPath("oras"); err != nil {
		t.Skip("oras CLI not available")
		return err
	}

	// Build list of files to push with RELATIVE paths
	var files []string

	// Add margo.yaml with custom media type (relative path)
	margoPath := "margo.yaml"
	if _, err := os.Stat(filepath.Join(pkgDir, margoPath)); err == nil {
		files = append(
			files,
			fmt.Sprintf(
				"%s:application/vnd.margo.app.description.v1+yaml",
				margoPath,
			),
		)
	} else {
		return fmt.Errorf("margo.yaml not found in %s", pkgDir)
	}

	// Add resources with octet-stream media type (relative paths)
	resourcesDir := filepath.Join(pkgDir, "resources")
	if info, err := os.Stat(resourcesDir); err == nil && info.IsDir() {
		err := filepath.Walk(
			resourcesDir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}

				// Get relative path from package directory
				relPath, err := filepath.Rel(pkgDir, path)
				if err != nil {
					return err
				}

				files = append(
					files,
					fmt.Sprintf("%s:application/octet-stream", relPath),
				)
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("failed to walk resources: %w", err)
		}
	}

	// Build oras push command
	reference := fmt.Sprintf("%s/%s:%s", registryURL, repo, tag)

	args := []string{
		"push",
		reference,
		"--artifact-type", "application/vnd.margo.app.v1+json",
		"--plain-http", // Force HTTP for test server
	}
	args = append(args, files...)

	// Execute oras push FROM the package directory (important!)
	cmd := exec.CommandContext(context.Background(), "oras", args...)
	cmd.Dir = pkgDir // Run from package directory so paths are relative

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"oras push failed: %w, output: %s",
			err,
			string(output),
		)
	}

	t.Logf("ORAS push output: %s", string(output))
	return nil
}
func TestOCILoader_WithMockRegistry(t *testing.T) {
	// Create in-memory OCI registry
	registryHandler := registry.New()
	server := httptest.NewServer(registryHandler)
	defer server.Close()

	// Create test package
	pkgDir := createValidTestPackage(t, true)
	defer os.RemoveAll(pkgDir)

	// Push package to mock registry
	registryURL := server.URL[7:]

	err := pushPackageToRegistry(
		t,
		pkgDir,
		registryURL,
		"test-package",
		"latest",
	)
	require.NoError(t, err)

	// Test loading from mock registry
	loader := NewOCILoader(defaultConfig())

	_, pkg, err := loader.Load(
		context.Background(),
		registryURL+"/test-package",
		&LoadOptions{
			Tag:      "latest",
			Insecure: true,
			IsHTTP:   true,
		},
	)

	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.Equal(t, "test-app", pkg.Description.Metadata.Name)
	assert.NotEmpty(t, pkg.Resources)
}

func TestOCILoader_parseSource(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantRegistry string
		wantRepo     string
	}{
		{
			name:         "standard format",
			source:       "docker.io/myuser/myapp",
			wantRegistry: "docker.io",
			wantRepo:     "myuser/myapp",
		},
		{
			name:         "with http protocol",
			source:       "http://localhost:5000/myapp",
			wantRegistry: "localhost:5000",
			wantRepo:     "myapp",
		},
		{
			name:         "with https protocol",
			source:       "https://ghcr.io/org/app",
			wantRegistry: "ghcr.io",
			wantRepo:     "org/app",
		},
		{
			name:         "registry only",
			source:       "localhost:5000",
			wantRegistry: "localhost:5000",
			wantRepo:     "",
		},
	}

	loader := NewOCILoader(defaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, repo := loader.parseSource(tt.source)
			assert.Equal(t, tt.wantRegistry, registry)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestOCILoader_Load(t *testing.T) {
	// Check if oras is available
	if _, err := exec.LookPath("oras"); err != nil {
		t.Skip("oras CLI not available")
	}

	tests := []struct {
		name        string
		source      string
		opts        *LoadOptions
		setupFunc   func(t *testing.T) string
		wantErr     bool
		errContains string
	}{
		{
			name:   "missing oras binary",
			source: "localhost:5000/test",
			opts: &LoadOptions{
				Tag: "latest",
			},
			wantErr:     true,
			errContains: "oras",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewOCILoader(defaultConfig())

			_, pkg, err := loader.Load(context.Background(), tt.source, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, pkg)
		})
	}
}

func TestOCILoader_Type(t *testing.T) {
	loader := NewOCILoader(defaultConfig())
	assert.Equal(t, SourceOCI, loader.Type())
}
