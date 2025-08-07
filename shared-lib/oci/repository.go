package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// RepositoryInfo contains metadata about a repository
type RepositoryInfo struct {
	Name        string            `json:"name"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations"`
}

// ListRepositories lists all repositories in the registry (if supported)
//
// Parameters:
//   - ctx: Context for the operation (required)
//
// Returns:
//   - []string: List of repository names
//   - error: An error if the operation fails
//
// Note: Not all registries support the catalog API
//
// Example:
//
//	repos, err := client.ListRepositories(ctx)
//	if err != nil {
//	    log.Printf("Registry may not support catalog API: %v", err)
//	    return
//	}
//	for _, repo := range repos {
//	    fmt.Printf("Repository: %s\n", repo)
//	}
func (c *Client) ListRepositories(ctx context.Context) ([]string, error) {
	// Create a registry reference
	registry, err := name.NewRegistry(c.config.Registry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry %s: %w", c.config.Registry, err)
	}
	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	// List repositories using catalog API
	repos, err := remote.Catalog(ctx, registry, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	return repos, nil
}

// GetRepositoryInfo retrieves detailed information about a repository
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - repository: The repository name
//
// Returns:
//   - *RepositoryInfo: Detailed information about the repository
//   - error: An error if the operation fails
//
// Example:
//
//	info, err := client.GetRepositoryInfo(ctx, "library/alpine")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Repository has %d tags\n", len(info.Tags))
func (c *Client) GetRepositoryInfo(ctx context.Context, repository string) (*RepositoryInfo, error) {
	if repository == "" {
		return nil, fmt.Errorf("repository cannot be empty")
	}
	// Get tags for the repository
	tags, err := c.ListTags(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tags: %w", err)
	}
	// Calculate repository size and get creation info
	var totalSize int64
	var createdAt, updatedAt time.Time
	annotations := make(map[string]string)
	// Get info from the latest tag if available
	if len(tags) > 0 {
		latestRef := fmt.Sprintf("%s/%s:%s", c.config.Registry, repository, tags[0])
		imageInfo, err := c.GetImageInfo(ctx, latestRef)
		if err == nil {
			totalSize = imageInfo.Size
			createdAt = imageInfo.CreatedAt
			updatedAt = imageInfo.CreatedAt
		}
	}
	return &RepositoryInfo{
		Name:        repository,
		Tags:        tags,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Size:        totalSize,
		Annotations: annotations,
	}, nil
}
