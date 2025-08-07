package packageManager

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/margo/dev-repo/non-standard/pkg/models"
	"github.com/margo/dev-repo/shared-lib/git"
	"gopkg.in/yaml.v3"
)

// ExpectedApplicationDescriptionFileName defines the standard expected filename for Margo application descriptions.
//
// This constant specifies the expected filename that contains the application metadata,
// configuration, and deployment information. The file should be located in the root
// directory of the application package and follow the Margo application description format.
//
// Example package structure:
//
//	my-app-pkg/
//	├── margo.yaml          // Application description file
//	├── resources/          // Optional resources directory
//	│   ├── icon.png
//	│   └── readme.md
const (
	ExpectedApplicationDescriptionFileName = "margo.yaml"
)

// PackageManager handles application package operations for Margo applications.
//
// This struct provides functionality to load, parse, and manage application packages
// from various sources including local directories and Git repositories. It handles
// the loading of application descriptions, resources, and validation of package structure.
//
// The PackageManager supports:
//   - Loading packages from local directories
//   - Loading packages from Git repositories (with optional authentication)
//   - Parsing Margo application description files
//   - Loading associated resources (icons, documentation, etc.)
//
// Example usage:
//
//	pm := NewPackageManager()
//	pkg, err := pm.LoadPackageFromDir("/path/to/package")
//	if err != nil {
//	    log.Fatal(err)
//	}
type PackageManager struct{}

// NewPackageManager creates a new PackageManager instance.
//
// This constructor function initializes a new PackageManager with default settings.
// The PackageManager is stateless and can be safely used concurrently across
// multiple goroutines.
//
// Returns:
//   - *PackageManager: A new PackageManager object instance
func NewPackageManager() *PackageManager {
	return &PackageManager{}
}

// LoadPackageFromGit loads an application package from a Git repository.
//
// then loads the application package from the cloned repository. It supports both
// public and private repositories through optional authentication.
//
// Parameters:
//   - url: The HTTPS Git repository URL containing the application package
//   - branchName: The name of the branch to clone (e.g., "main", "develop")
//   - subPath: If the app description file is not present at root level, then provide its path within the repo (e.g., "app-pkgs/pkg1")
//   - auth: Optional authentication credentials for private repositories (can be nil)
//
// Returns:
//   - pkgPath: The absolute path to the cloned package directory
//   - pkg: The loaded application package with description and resources
//   - err: An error if the clone or load operation fails
//
// Important Notes:
//   - The caller is responsible for cleaning up the returned pkgPath directory
//   - Only HTTPS-based Git URLs are supported; SSH URLs are not supported
//   - The repository must contain a valid margo.yaml file in its root directory
//   - Resources directory is optional and will be loaded if present
//
// Example:
//
//	pm := NewPackageManager()
//	auth := &git.Auth{Username: "user", Token: "token"}
//	pkgPath, pkg, err := pm.LoadPackageFromGit("https://github.com/user/app.git", "main", auth)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer os.RemoveAll(pkgPath) // Clean up temporary directory
//
// Errors:
//   - Returns error if Git clone operation fails
//   - Returns error if package loading from directory fails
//   - Returns error if margo.yaml file is missing or invalid
func (pm *PackageManager) LoadPackageFromGit(url, branchName, subPath string, auth *git.Auth) (pkgPath string, pkg *models.AppPkg, err error) {
	// Clone repository to temporary directory
	gitClient, err := git.NewClient(auth, url, branchName, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to initialize git client: %w", err)
	}

	dirPath, err := gitClient.Clone(nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	if subPath != "" {
		dirPath += "/" + subPath
	}

	// Load package from cloned directory
	appPackage, err := pm.LoadPackageFromDir(dirPath)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(dirPath)
		return "", nil, fmt.Errorf("failed to load package from cloned repository: %w", err)
	}

	return dirPath, appPackage, nil
}

// LoadPackageFromDir loads an application package from a local directory.
//
// This method loads a Margo application package from the specified directory path.
// It searches for the application description file (margo.yaml), parses it, and
// loads any associated resources from the resources subdirectory.
//
// Parameters:
//   - pkgPath: The absolute or relative path to the package directory
//
// Returns:
//   - *models.AppPkg: The loaded application package with description and resources
//   - error: An error if the package cannot be loaded or is invalid
//
// Expected package structure:
//
//	package-directory/
//	├── margo.yaml          // Required: Application description
//	└── resources/          // Optional: Resources directory
//	    ├── icon.png        // Optional: Application icon
//	    ├── description.md  // Optional: Detailed description
//	    └── license.txt     // Optional: License file
//
// Loading behavior:
//   - The margo.yaml file is required and must be valid
//   - The resources directory is optional; if present, all files are loaded
//   - Resource files are loaded as byte arrays and stored in the package
//   - Subdirectories within resources are not recursively processed
//
// Example:
//
//	pm := NewPackageManager()
//	pkg, err := pm.LoadPackageFromDir("/path/to/my-app")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Loaded app: %s v%s\n", pkg.Description.Metadata.Name, pkg.Description.Metadata.Version)
//
// Errors:
//   - Returns error if pkgPath does not exist or is not accessible
//   - Returns error if margo.yaml file is missing, unreadable, or invalid
//   - Returns error if resources directory exists but cannot be read
func (pm *PackageManager) LoadPackageFromDir(pkgPath string) (*models.AppPkg, error) {
	// Validate package path exists
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("package directory does not exist: %s", pkgPath)
	}

	// Initialize package with empty resources map
	pkg := &models.AppPkg{Resources: make(map[string][]byte)}

	// Find and load application description
	descFile, err := pm.findAppDescription(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find application description: %w", err)
	}

	pkg.Description, err = pm.loadAppDescription(descFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load application description: %w", err)
	}

	// Load resources if directory exists
	resourcesPath := filepath.Join(pkgPath, "resources")
	if info, err := os.Stat(resourcesPath); err == nil && info.IsDir() {
		if err := pm.loadAppResources(resourcesPath, pkg.Resources); err != nil {
			return nil, fmt.Errorf("failed to load resources: %w", err)
		}
	}

	return pkg, nil
}

// findAppDescription finds the application description file in the package root directory.
//
// Parameters:
//   - pkgPath: The absolute or relative path to the package directory to search
//
// Returns:
//   - string: The absolute path to the valid application description file
//   - error: An error if no valid application description file is found or if directory traversal fails
//
// Search behavior:
//   - Only searches files in the root directory (not subdirectories)
//   - Looks for files matching the expected filename (case-insensitive)
//   - Validates that found files contain 'kind: ApplicationDescription'
//   - Returns the first valid application description file found
//
// Example:
//
//	descPath, err := pm.findAppDescription("/path/to/package")
//	if err != nil {
//	    log.Fatal("No valid application description found:", err)
//	}
//
// Errors:
//   - Returns error if directory traversal fails
//   - Returns error if no ApplicationDescription file is found in package root
func (pm *PackageManager) findAppDescription(pkgPath string) (string, error) {
	var candidates []string

	// Walk only the root directory to find application description files
	err := filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to access path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only check files in the root directory
		if filepath.Dir(path) != pkgPath {
			return nil
		}

		// Check for expected application description filename (case-insensitive)
		if strings.EqualFold(info.Name(), ExpectedApplicationDescriptionFileName) {
			candidates = append(candidates, path)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to search package directory: %w", err)
	}

	// Validate each candidate file contains ApplicationDescription
	for _, candidate := range candidates {
		if pm.isValidAppDescription(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no valid ApplicationDescription file (%s) found in package root: %s",
		ExpectedApplicationDescriptionFileName, pkgPath)
}

// isValidAppDescription checks if a YAML file contains a valid ApplicationDescription.
//
// Parameters:
//   - filePath: The absolute path to the YAML file to validate
//
// Returns:
//   - bool: true if the file contains a valid ApplicationDescription, false otherwise
//
// Validation criteria:
//   - File must be readable
//   - File must contain valid YAML
//   - YAML must have 'kind: ApplicationDescription' field
//
// Example:
//
//	if pm.isValidAppDescription("/path/to/margo.yaml") {
//	    fmt.Println("Valid application description file")
//	}
//
// Note: This method returns false for any error condition (file not found, invalid YAML, etc.)
// to allow graceful handling during file discovery.
func (pm *PackageManager) isValidAppDescription(filePath string) bool {
	// Read file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	// Parse YAML to check kind field
	var doc struct {
		Kind string `yaml:"kind"`
	}

	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}

	// Validate kind field matches expected value
	return strings.EqualFold(doc.Kind, "ApplicationDescription")
}

// loadAppDescription loads and parses an application description file.
//
// This method opens the specified application description file, parses it using the
// models package parser, and returns a structured ApplicationDescription object.
// It handles file I/O and YAML parsing, providing detailed error information for
// debugging purposes.
//
// Parameters:
//   - filePath: The absolute path to the application description file to load
//
// Returns:
//   - *models.ApplicationDescription: The parsed application description object
//   - error: An error if the file cannot be opened, read, or parsed
//
// Loading process:
//   - Opens the file for reading
//   - Uses models.ParseApplicationDescription with YAML format
//   - Returns structured ApplicationDescription object
//   - Future: Will include validation of required fields
//
// Example:
//
//	desc, err := pm.loadAppDescription("/path/to/margo.yaml")
//	if err != nil {
//	    log.Fatal("Failed to load application description:", err)
//	}
//	fmt.Printf("Loaded app: %s v%s\n", desc.Metadata.Name, desc.Metadata.Version)
//
// Errors:
//   - Returns error if file cannot be opened or read
//   - Returns error if YAML parsing fails
//   - Returns error if application description format is invalid
//   - Future: Will return validation errors for missing required fields
func (pm *PackageManager) loadAppDescription(filePath string) (*models.ApplicationDescription, error) {
	// Open file for reading
	reader, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open application description file %s: %w", filePath, err)
	}
	defer reader.Close()

	// Parse application description using models package
	desc, err := models.ParseApplicationDescription(reader, models.ApplicationDescriptionFormatYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse application description from %s: %w", filePath, err)
	}

	// TODO: Add comprehensive validation
	// Validate required fields and structure
	// if err := pm.validateApplicationDescription(&desc); err != nil {
	// 	return nil, fmt.Errorf("application description validation failed: %w", err)
	// }

	return &desc, nil
}

// loadAppResources loads all files from the resources directory into memory.
//
// This method recursively walks through the resources directory and loads all files
// into the provided resources map. Files are stored as byte arrays with their relative
// paths as keys. This allows the package to be self-contained with all resources
// embedded within the package structure.
//
// Parameters:
//   - resourcesPath: The absolute path to the resources directory to load from
//   - resources: A map to store loaded resources (key: relative path, value: file content)
//
// Returns:
//   - error: An error if directory traversal or file reading fails
//
// Loading behavior:
//   - Recursively processes all files in the resources directory and subdirectories
//   - Stores files using their relative path from the resources directory as the key
//   - Skips directories (only loads actual files)
//   - Overwrites existing entries in the resources map if keys conflict
//
// Example:
//
//	resources := make(map[string][]byte)
//	err := pm.loadAppResources("/path/to/resources", resources)
//	if err != nil {
//	    log.Fatal("Failed to load resources:", err)
//	}
//	// resources["icon.png"] contains the icon file content
//	// resources["docs/readme.md"] contains the readme file content
//
// Errors:
//   - Returns error if resources directory cannot be accessed
//   - Returns error if any file cannot be read
//   - Returns error if relative path calculation fails
func (pm *PackageManager) loadAppResources(resourcesPath string, resources map[string][]byte) error {
	return filepath.Walk(resourcesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to access path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from resources directory
		relPath, err := filepath.Rel(resourcesPath, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path for %s: %w", path, err)
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read resource file %s: %w", path, err)
		}

		// Store resource with relative path as key
		resources[relPath] = content
		return nil
	})
}

// CreatePackage creates a new application package directory structure.
//
// This method creates a complete Margo application package at the specified output path.
// It writes the application description as margo.yaml and creates a resources directory
// with all provided resource files. The created package follows the standard Margo
// package structure and can be loaded by LoadPackageFromDir.
//
// Parameters:
//   - desc: The application description to write as margo.yaml
//   - resources: A map of resource files (key: relative path, value: file content)
//   - outputPath: The directory path where the package should be created
//
// Returns:
//   - error: An error if package creation fails at any step
//
// Created package structure:
//
//	outputPath/
//	├── margo.yaml          // Application description
//	└── resources/          // Resources directory (created only if resources provided)
//	    ├── icon.png        // Resource files
//	    └── docs/
//	        └── readme.md   // Subdirectories are created as needed
//
// Creation behavior:
//   - Creates the output directory if it doesn't exist
//   - Writes application description as YAML to margo.yaml
//   - Creates resources directory only if resources are provided
//   - Creates subdirectories for resources as needed
//   - Overwrites existing files if they exist
//
// Example:
//
//	desc := models.ApplicationDescription{...}
//	resources := map[string][]byte{
//	    "icon.png": iconData,
//	    "docs/readme.md": readmeData,
//	}
//	err := pm.CreatePackage(desc, resources, "/path/to/new-package")
//	if err != nil {
//	    log.Fatal("Failed to create package:", err)
//	}
//
// Errors:
//   - Returns error if output directory cannot be created
//   - Returns error if application description marshaling fails
//   - Returns error if margo.yaml file cannot be written
//   - Returns error if resources directory cannot be created
//   - Returns error if any resource file cannot be written
func (pm *PackageManager) CreatePackage(desc models.ApplicationDescription, resources map[string][]byte, outputPath string) error {
	// Create package directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create package directory %s: %w", outputPath, err)
	}

	// Write application description
	descData, err := yaml.Marshal(desc)
	if err != nil {
		return fmt.Errorf("failed to marshal application description: %w", err)
	}

	descFile := filepath.Join(outputPath, ExpectedApplicationDescriptionFileName)
	if err := os.WriteFile(descFile, descData, 0644); err != nil {
		return fmt.Errorf("failed to write application description to %s: %w", descFile, err)
	}

	// Create resources directory and files if resources are provided
	if len(resources) > 0 {
		resourcesDir := filepath.Join(outputPath, "resources")
		if err := os.MkdirAll(resourcesDir, 0755); err != nil {
			return fmt.Errorf("failed to create resources directory %s: %w", resourcesDir, err)
		}

		// Write resource files
		for filename, content := range resources {
			resourcePath := filepath.Join(resourcesDir, filename)

			// Create subdirectories if needed
			resourceDir := filepath.Dir(resourcePath)
			if err := os.MkdirAll(resourceDir, 0755); err != nil {
				return fmt.Errorf("failed to create resource subdirectory %s: %w", resourceDir, err)
			}

			if err := os.WriteFile(resourcePath, content, 0644); err != nil {
				return fmt.Errorf("failed to write resource file %s: %w", filename, err)
			}
		}
	}

	return nil
}

// PackageToTarball creates a compressed tarball (.tar.gz) from an application package.
//
// This method creates a gzip-compressed tar archive containing the application description
// and all resources from the provided package. The resulting tarball can be distributed,
// stored, or deployed as a single file. The tarball maintains the standard Margo package
// structure with margo.yaml in the root and resources in the resources/ directory.
//
// Parameters:
//   - pkg: The application package to archive
//   - outputPath: The file path where the tarball should be created (should end with .tar.gz)
//
// Returns:
//   - error: An error if tarball creation fails at any step
//
// Tarball structure:
//
//	archive.tar.gz
//	├── margo.yaml          // Application description
//	└── resources/          // Resources directory
//	    ├── icon.png        // Resource files
//	    └── docs/readme.md  // Maintains relative paths
//
// Creation process:
//   - Creates output file with gzip compression
//   - Adds application description as margo.yaml
//   - Adds all resources maintaining their relative paths
//   - Uses standard tar format compatible with most tools
//
// Example:
//
//	pkg := &models.AppPkg{...}
//	err := pm.PackageToTarball(pkg, "/path/to/package.tar.gz")
//	if err != nil {
//	    log.Fatal("Failed to create tarball:", err)
//	}
//
// Errors:
//   - Returns error if output file cannot be created
//   - Returns error if application description marshaling fails
//   - Returns error if tar header writing fails
//   - Returns error if file content writing fails
//
// Note: The caller should ensure the output directory exists and is writable.
func (pm *PackageManager) PackageToTarball(pkg *models.AppPkg, outputPath string) error {
	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file %s: %w", outputPath, err)
	}
	defer file.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add application description
	descData, err := yaml.Marshal(pkg.Description)
	if err != nil {
		return fmt.Errorf("failed to marshal application description: %w", err)
	}

	descHeader := &tar.Header{
		Name: ExpectedApplicationDescriptionFileName,
		Mode: 0644,
		Size: int64(len(descData)),
	}

	if err := tarWriter.WriteHeader(descHeader); err != nil {
		return fmt.Errorf("failed to write application description header: %w", err)
	}

	if _, err := tarWriter.Write(descData); err != nil {
		return fmt.Errorf("failed to write application description content: %w", err)
	}

	// Add resources
	for filename, content := range pkg.Resources {
		resourceHeader := &tar.Header{
			Name: filepath.Join("resources", filename),
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tarWriter.WriteHeader(resourceHeader); err != nil {
			return fmt.Errorf("failed to write resource header for %s: %w", filename, err)
		}

		if _, err := tarWriter.Write(content); err != nil {
			return fmt.Errorf("failed to write resource content for %s: %w", filename, err)
		}
	}

	return nil
}

func (pm *PackageManager) checkPkgUpdates(pkg *models.AppPkg) error {
	return nil
}
