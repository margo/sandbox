package packageManager

// // Concrete source implementations
// type GitPackageSource struct {
// 	URL         string
// 	Branch      string
// 	Auth        *GitAuth
// 	registryURL string
// }

// // Factory functions
// func NewGitPackageManager(registryURL, gitURL, branch string, auth *GitAuth) *PackageManager[GitConfig, *GitPackageSource] {
// 	source := &GitPackageSource{
// 		URL:         gitURL,
// 		Branch:      branch,
// 		Auth:        auth,
// 		registryURL: registryURL,
// 	}
// 	return NewPackageManager[GitConfig](source)
// }

// func (g *GitPackageSource) Load(ctx context.Context) (*models.ApplicationPackage, error) {
// 	// Git loading implementation
// 	return &models.ApplicationPackage{}, nil
// }

// func (g *GitPackageSource) Validate() error {
// 	if g.URL == "" {
// 		return fmt.Errorf("git URL is required")
// 	}
// 	return nil
// }

// func (g *GitPackageSource) GetMetadata() GitConfig {
// 	return GitConfig{URL: g.URL, Branch: g.Branch, Auth: g.Auth}
// }
