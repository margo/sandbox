package packageManager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/margo/sandbox/non-standard/generatedCode/wfm/nbi"
	"github.com/margo/sandbox/non-standard/pkg/models"
)

const ExpectedDescriptionFileName = "margo.yaml"

// DirectoryLoader loads packages from local directories
type DirectoryLoader struct {
	config *Config
}

func NewDirectoryLoader(config *Config) *DirectoryLoader {
	return &DirectoryLoader{config: config}
}

func (l *DirectoryLoader) Type() SourceType {
	return SourceDirectory
}

func (l *DirectoryLoader) Load(
	ctx context.Context,
	path string,
	opts *LoadOptions,
) (string /*path*/, *models.AppPkg, error) {
	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil, &ErrPackageNotFound{Path: path}
	}

	pkg := &models.AppPkg{Resources: make(map[string][]byte)}

	// Find and load description
	descPath, err := l.findDescription(path)
	if err != nil {
		return "", nil, err
	}

	pkg.Description, err = l.loadDescription(descPath)
	if err != nil {
		return "", nil, err
	}

	if opts != nil && opts.Validate {
		if pkg.Description.Kind != "ApplicationDescription" {
			return "", nil, &ErrInvalidDescription{
				Path: descPath,
				ContextualInfo: fmt.Sprintf(
					"invalid kind: %s",
					pkg.Description.Kind,
				),
			}
		}

		if pkg.Description.ApiVersion != "margo.org/v1-alpha1" {
			return "", nil, &ErrInvalidDescription{
				Path: descPath,
				ContextualInfo: fmt.Sprintf(
					"invalid apiversion: %s",
					pkg.Description.ApiVersion,
				),
			}
		}
	}

	// Load resources if they exist
	resourcesPath := filepath.Join(path, "resources")
	if info, err := os.Stat(resourcesPath); err == nil && info.IsDir() {
		if err := l.loadResources(resourcesPath, pkg.Resources); err != nil {
			return "", nil, err
		}
	}

	return path, pkg, nil
}

func (l *DirectoryLoader) findDescription(pkgPath string) (string, error) {
	descPath := filepath.Join(pkgPath, ExpectedDescriptionFileName)

	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		return "", &ErrDescriptionNotFound{Path: pkgPath}
	}

	// Validate it's actually an ApplicationDescription
	if !l.isValidDescription(descPath) {
		return "", &ErrInvalidDescription{Path: descPath}
	}

	return descPath, nil
}

func (l *DirectoryLoader) isValidDescription(path string) bool {
	if _, err := l.loadDescription(path); err != nil {
		return false
	}
	return true
}

func (l *DirectoryLoader) loadDescription(
	path string,
) (*nbi.AppDescription, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open description: %w", err)
	}
	defer file.Close()

	desc, err := models.ParseApplicationDescription(
		file,
		models.ApplicationDescriptionFormatYAML,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse description: %w", err)
	}

	return &desc, nil
}

func (l *DirectoryLoader) loadResources(
	resourcesPath string,
	resources map[string][]byte,
) error {
	// 1. Open the secure root
	root, err := os.OpenRoot(filepath.Clean(resourcesPath))
	if err != nil {
		return fmt.Errorf("failed to open app resources root: %w", err)
	}
	defer func() {
		_ = root.Close()
	}()

	// 2. Get the FS interface from the root
	rootFS := root.FS()

	// 3. Walk the FS (starting at "." which is the root directory)
	return fs.WalkDir(
		rootFS,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			// 4. Use fs.ReadFile on the restricted filesystem
			content, err := fs.ReadFile(rootFS, path)
			if err != nil {
				return fmt.Errorf("failed to read resource %s: %w", path, err)
			}

			resources[path] = content
			return nil
		},
	)
}
