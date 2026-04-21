package packageManager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/sandbox/non-standard/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPackageManager(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name: "with custom config",
			config: &Config{
				EnableValidation: false,
				MaxPackageSize:   50 * 1024 * 1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPackageManager(tt.config)

			assert.NotNil(t, pm)
			assert.NotNil(t, pm.loaders)
			assert.NotNil(t, pm.validator)
			assert.NotNil(t, pm.config)

			// Verify loaders are registered
			assert.Len(t, pm.loaders, 2)
			assert.NotNil(t, pm.loaders[SourceDirectory])
			assert.NotNil(t, pm.loaders[SourceOCI])
		})
	}
}

func TestPackageManager_RegisterLoader(t *testing.T) {
	pm := NewPackageManager(nil)

	// Create a custom loader
	customLoader := &mockLoader{sourceType: "custom"}
	pm.RegisterLoader(customLoader)

	assert.NotNil(t, pm.loaders["custom"])
}

func TestPackageManager_Load_UnsupportedSourceType(t *testing.T) {
	pm := NewPackageManager(nil)

	_, pkg, err := pm.Load(
		context.Background(),
		"unsupported",
		"/some/path",
		nil,
	)

	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.IsType(t, &ErrUnsupportedSource{}, err)
}

func TestPackageManager_Load_FromDirectory(t *testing.T) {
	pkgDir := createValidTestPackage(t, true)
	defer os.RemoveAll(pkgDir)

	pm := NewPackageManager(nil)
	_, pkg, err := pm.Load(
		context.Background(),
		SourceDirectory,
		pkgDir,
		nil,
	)

	require.NoError(t, err)
	assert.NotNil(t, pkg)
}

func TestPackageManager_Load_WithValidationEnabled(t *testing.T) {
	pkgDir := createValidTestPackage(t, true)
	defer os.RemoveAll(pkgDir)

	pm := NewPackageManager(nil)
	_, pkg, err := pm.Load(
		context.Background(),
		SourceDirectory,
		pkgDir,
		&LoadOptions{
			Validate: true,
		},
	)

	require.NoError(t, err)
	assert.NotNil(t, pkg)
}

func TestPackageManager_Load_InvalidPackageWithValidation(t *testing.T) {
	dir := t.TempDir()
	invalidYAML := `
apiVersion: margo.org/v1alpha1
kind: WrongKind
metadata:
  name: test
`
	err := os.WriteFile(
		filepath.Join(dir, ExpectedDescriptionFileName),
		[]byte(invalidYAML),
		0600,
	)
	require.NoError(t, err)

	pm := NewPackageManager(nil)
	_, pkg, err := pm.Load(
		context.Background(),
		SourceDirectory,
		dir,
		&LoadOptions{
			Validate: true,
		},
	)

	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.IsType(t, &ErrInvalidDescription{}, err)
}

func TestPackageManager_LoadFromDirectory(t *testing.T) {
	pkgDir := createValidTestPackage(t, true)
	defer os.RemoveAll(pkgDir)

	pm := NewPackageManager(nil)
	_, pkg, err := pm.LoadFromDirectory(context.Background(), pkgDir)

	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.NotNil(t, pkg.Description)
	assert.Equal(t, "test-app", pkg.Description.Metadata.Name)
}

func TestPackageManager_LoadFromDirectory_InvalidPath(t *testing.T) {
	pm := NewPackageManager(nil)
	_, pkg, err := pm.LoadFromDirectory(
		context.Background(),
		"/non/existent/path",
	)

	assert.Error(t, err)
	assert.Nil(t, pkg)
}

func TestNewPackageManager_WithNilConfig(t *testing.T) {
	pm := NewPackageManager(nil)

	assert.NotNil(t, pm)
	assert.NotNil(t, pm.loaders)
	assert.NotNil(t, pm.validator)
	assert.NotNil(t, pm.config)
	assert.True(t, pm.config.EnableValidation)
}

func TestNewPackageManager_WithCustomConfig(t *testing.T) {
	customConfig := &Config{
		EnableValidation: false,
		MaxPackageSize:   50 * 1024 * 1024,
	}

	pm := NewPackageManager(customConfig)

	assert.NotNil(t, pm)
	assert.False(t, pm.config.EnableValidation)
	assert.Equal(t, int64(50*1024*1024), pm.config.MaxPackageSize)
}

func TestPackageManager_LoadFromOCI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OCI integration test")
	}

	pm := NewPackageManager(nil)

	_, _, err := pm.LoadFromOCI(
		context.Background(),
		"localhost:5000",
		"test-package",
		"latest",
		"",
		"",
		true,
		time.Second*30,
	)

	// Expected to fail without real registry
	assert.Error(t, err)
}

// Mock loader for testing
type mockLoader struct {
	sourceType SourceType
}

func (m *mockLoader) Load(
	ctx context.Context,
	source string,
	opts *LoadOptions,
) (string, *models.AppPkg, error) {
	return "", &models.AppPkg{}, nil
}

func (m *mockLoader) Type() SourceType {
	return m.sourceType
}
