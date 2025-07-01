package packageManager

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/margo/dev-repo/sdk/pkg/models"
	"gopkg.in/yaml.v3"
)

const (
	ExpectedApplicationDescriptionFileName = "margo.yaml"
)

// PackageManager handles application package operations
type PackageManager struct{}

// NewPackageManager creates a new package manager
func NewPackageManager() *PackageManager {
	return &PackageManager{}
}

// LoadPackage loads an application package from a directory
func (pm *PackageManager) LoadPackage(packagePath string) (*models.ApplicationPackage, error) {
	pkg := &models.ApplicationPackage{Resources: make(map[string][]byte), RootPath: packagePath}

	// Find and load application description
	descFile, err := pm.findApplicationDescription(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find application description: %w", err)
	}

	pkg.Description, err = pm.loadApplicationDescription(descFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load application description: %w", err)
	}

	// Load resources
	resourcesPath := filepath.Join(packagePath, "resources")
	if _, err := os.Stat(resourcesPath); err == nil {
		err = pm.loadResources(resourcesPath, pkg.Resources)
		if err != nil {
			return nil, fmt.Errorf("failed to load resources: %w", err)
		}
	}

	return pkg, nil
}

// findApplicationDescription finds the application description file
func (pm *PackageManager) findApplicationDescription(packagePath string) (string, error) {
	var candidates []string

	err := filepath.Walk(packagePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only check files in the root directory
		if filepath.Dir(path) != packagePath {
			return nil
		}

		// if strings.EqualFold(strings.ToLower(info.Name()), ".yaml") ||
		// 	strings.HasSuffix(strings.ToLower(info.Name()), ".yml") {
		// 	candidates = append(candidates, path)
		// }

		if strings.EqualFold(strings.ToLower(info.Name()), ExpectedApplicationDescriptionFileName) {
			candidates = append(candidates, path)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// Check each candidate for ApplicationDescription kind
	for _, candidate := range candidates {
		if isApplicationDescription(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no ApplicationDescription file found in package root")
}

// isApplicationDescription checks if a YAML file contains ApplicationDescription
func isApplicationDescription(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	var doc struct {
		Kind string `yaml:"kind"`
	}

	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}

	return doc.Kind == "ApplicationDescription"
}

// loadApplicationDescription loads and validates application description
func (pm *PackageManager) loadApplicationDescription(filePath string) (*models.ApplicationDescription, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var desc models.ApplicationDescription
	if err := yaml.Unmarshal(data, &desc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	// TODO: finish validations
	// if err := pm.validateApplicationDescription(&desc); err != nil {
	// 	return nil, fmt.Errorf("validation failed: %w", err)
	// }

	return &desc, nil
}

// loadResources loads all files from the resources directory
func (pm *PackageManager) loadResources(resourcesPath string, resources map[string][]byte) error {
	return filepath.Walk(resourcesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path from resources directory
		relPath, err := filepath.Rel(resourcesPath, path)
		if err != nil {
			return err
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		resources[relPath] = content
		return nil
	})
}

// // validateDeploymentProfile validates a deployment profile
// func (pm *PackageManager) validateDeploymentProfile(profile *models.DeploymentProfile) error {
// 	if profile.Name == "" {
// 		return fmt.Errorf("deployment profile name is required")
// 	}

// 	switch profile.Type {
// 	case DeploymentProfileTypeHelm:
// 		if profile.Helm == nil {
// 			return fmt.Errorf("helm configuration is required for helm deployment profile")
// 		}
// 		if profile.Helm.ChartName == "" {
// 			return fmt.Errorf("helm chart name is required")
// 		}
// 	case DeploymentProfileTypeCompose:
// 		if profile.Compose == nil {
// 			return fmt.Errorf("compose configuration is required for compose deployment profile")
// 		}
// 		if profile.Compose.ComposeFile == "" {
// 			return fmt.Errorf("compose file is required")
// 		}
// 	case DeploymentProfileTypeBoth:
// 		if profile.Helm == nil || profile.Compose == nil {
// 			return fmt.Errorf("both helm and compose configurations are required for 'both' deployment profile")
// 		}
// 	default:
// 		return fmt.Errorf("invalid deployment profile type: %s", profile.Type)
// 	}

// 	return nil
// }

// CreatePackage creates a new application package
func (pm *PackageManager) CreatePackage(desc models.ApplicationDescription, resources map[string][]byte, outputPath string) error {
	// Create package directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create package directory: %w", err)
	}

	// Write application description
	descData, err := yaml.Marshal(desc)
	if err != nil {
		return fmt.Errorf("failed to marshal application description: %w", err)
	}

	descFile := filepath.Join(outputPath, "margo.yaml")
	if err := os.WriteFile(descFile, descData, 0644); err != nil {
		return fmt.Errorf("failed to write application description: %w", err)
	}

	// Create resources directory if needed
	if len(resources) > 0 {
		resourcesDir := filepath.Join(outputPath, "resources")
		if err := os.MkdirAll(resourcesDir, 0755); err != nil {
			return fmt.Errorf("failed to create resources directory: %w", err)
		}

		// Write resource files
		for filename, content := range resources {
			resourcePath := filepath.Join(resourcesDir, filename)

			// Create subdirectories if needed
			if err := os.MkdirAll(filepath.Dir(resourcePath), 0755); err != nil {
				return fmt.Errorf("failed to create resource subdirectory: %w", err)
			}

			if err := os.WriteFile(resourcePath, content, 0644); err != nil {
				return fmt.Errorf("failed to write resource file %s: %w", filename, err)
			}
		}
	}

	return nil
}

// PackageToTarball creates a tarball from an application package
func (pm *PackageManager) PackageToTarball(pkg *models.ApplicationPackage, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add application description
	descData, err := yaml.Marshal(pkg.Description)
	if err != nil {
		return fmt.Errorf("failed to marshal application description: %w", err)
	}

	header := &tar.Header{
		Name: "margo.yaml",
		Mode: 0644,
		Size: int64(len(descData)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tarWriter.Write(descData); err != nil {
		return err
	}

	// Add resources
	for filename, content := range pkg.Resources {
		header := &tar.Header{
			Name: filepath.Join("resources", filename),
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if _, err := tarWriter.Write(content); err != nil {
			return err
		}
	}

	return nil
}

// LoadFromTarball loads an application package from a tarball
func (pm *PackageManager) LoadFromTarball(tarballPath string) (*models.ApplicationPackage, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tarball: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	pkg := &models.ApplicationPackage{
		Resources: make(map[string][]byte),
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file content: %w", err)
		}

		if strings.HasSuffix(header.Name, ".yaml") || strings.HasSuffix(header.Name, ".yml") {
			// Check if this is the application description
			var doc struct {
				Kind string `yaml:"kind"`
			}

			if yaml.Unmarshal(content, &doc) == nil && doc.Kind == "ApplicationDescription" {
				var desc models.ApplicationDescription
				if err := yaml.Unmarshal(content, &desc); err != nil {
					return nil, fmt.Errorf("failed to parse application description: %w", err)
				}
				pkg.Description = &desc
				continue
			}
		}

		// Add to resources
		if strings.HasPrefix(header.Name, "resources/") {
			resourceName := strings.TrimPrefix(header.Name, "resources/")
			pkg.Resources[resourceName] = content
		}
	}

	if pkg.Description == nil {
		return nil, fmt.Errorf("no application description found in tarball")
	}

	return pkg, nil
}
