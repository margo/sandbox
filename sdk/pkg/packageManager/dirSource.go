package packageManager

// type DirectoryPackageSource struct {
// 	Path        string
// 	Recursive   bool
// 	registryURL string
// }

// func NewDirectoryPackageManager(registryURL, path string, recursive bool) *PackageManager[DirectoryConfig, *DirectoryPackageSource] {
// 	source := &DirectoryPackageSource{
// 		Path:        path,
// 		Recursive:   recursive,
// 		registryURL: registryURL,
// 	}
// 	return NewPackageManager[DirectoryConfig](source)
// }

// func (d *DirectoryPackageSource) Load(ctx context.Context) (*ApplicationPackage, error) {
// 	// Directory loading implementation
// 	return &models.ApplicationPackage{}, nil
// }

// func (d *DirectoryPackageSource) Validate() error {
// 	if d.Path == "" {
// 		return fmt.Errorf("directory path is required")
// 	}
// 	return nil
// }

// func (d *DirectoryPackageSource) GetMetadata() DirectoryConfig {
// 	return DirectoryConfig{Path: d.Path, Recursive: d.Recursive}
// }
