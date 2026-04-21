package packageManager

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/sandbox/non-standard/pkg/models"
)

// PackageManager handles all package operations
type PackageManager struct {
	loaders   map[SourceType]Loader
	validator *Validator
	config    *Config
}

// NewPackageManager creates a new package manager
func NewPackageManager(config *Config) *PackageManager {
	if config == nil {
		config = defaultConfig()
	}

	pm := &PackageManager{
		loaders:   make(map[SourceType]Loader),
		validator: NewValidator(),
		config:    config,
	}

	// Register loaders
	pm.RegisterLoader(NewDirectoryLoader(config))
	pm.RegisterLoader(NewOCILoader(config))

	return pm
}

// RegisterLoader registers a custom loader
func (pm *PackageManager) RegisterLoader(loader Loader) {
	pm.loaders[loader.Type()] = loader
}

// Load loads a package from any source
func (pm *PackageManager) Load(
	ctx context.Context,
	sourceType SourceType,
	source string,
	opts *LoadOptions,
) (string, *models.AppPkg, error) {
	loader, ok := pm.loaders[sourceType]
	if !ok {
		return "", nil, &ErrUnsupportedSource{Type: sourceType}
	}

	if opts == nil {
		opts = &LoadOptions{Timeout: DefaultTimeout}
	}

	path, pkg, err := loader.Load(ctx, source, opts)
	if err != nil {
		return "", nil, err
	}

	// Validate if enabled
	if pm.config.EnableValidation || opts.Validate {
		if err := pm.validator.Validate(ctx, pkg); err != nil {
			return "", nil, &ErrValidation{Err: err}
		}
	}

	return path, pkg, nil
}

// LoadFromDirectory is a convenience method
func (pm *PackageManager) LoadFromDirectory(
	ctx context.Context,
	path string,
) (string /*path*/, *models.AppPkg, error) {
	return pm.Load(ctx, SourceDirectory, path, nil)
}

// LoadFromOCI is a convenience method
func (pm *PackageManager) LoadFromOCI(
	ctx context.Context,
	registry, repo, tag, username, token string,
	insecure bool,
	timeout time.Duration,
) (string /*path*/, *models.AppPkg, error) {
	source := fmt.Sprintf("%s/%s", registry, repo)
	return pm.Load(ctx, SourceOCI, source, &LoadOptions{
		Tag:      tag,
		Username: username,
		Token:    token,
		Insecure: insecure,
		Timeout:  timeout,
	})
}

func defaultConfig() *Config {
	return &Config{
		TempDir:          "",
		EnableValidation: true,
		MaxPackageSize:   100 * 1024 * 1024, // 100MB
		DirPermissions:   DefaultDirPermissions,
		FilePermissions:  DefaultFilePermissions,
	}
}
