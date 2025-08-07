package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ImageInfo contains metadata about an image/artifact
type ImageInfo struct {
	Reference string    `json:"reference"`
	Digest    string    `json:"digest"`
	MediaType string    `json:"media_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	// Annotations  map[string]string `json:"annotations"`
	Labels       map[string]string `json:"labels"`
	Architecture string            `json:"architecture"`
	OS           string            `json:"os"`
}

// PushResult contains information about a successful push operation
type PushResult struct {
	Reference string    `json:"reference"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	PushedAt  time.Time `json:"pushed_at"`
}

// PullResult contains information about a successful pull operation
type PullResult struct {
	Reference string    `json:"reference"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	PulledAt  time.Time `json:"pulled_at"`
}

// PushImage pushes an image to the OCI registry
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - image: The image to push (required)
//   - reference: The target reference (e.g., "registry.io/user/repo:tag")
//
// Returns:
//   - *PushResult: Information about the pushed image
//   - error: An error if the push operation fails
//
// Example:
//
//	result, err := client.PushImage(ctx, image, "docker.io/myuser/myapp:v1.0.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Pushed image with digest: %s\n", result.Digest)
func (c *Client) PushImage(ctx context.Context, image v1.Image, reference string) (*PushResult, error) {
	if image == nil {
		return nil, fmt.Errorf("image cannot be nil")
	}
	if reference == "" {
		return nil, fmt.Errorf("reference cannot be empty")
	}

	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}

	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))

	// Push the image
	startTime := time.Now()
	if err := remote.Write(ref, image, opts...); err != nil {
		return nil, fmt.Errorf("failed to push image to %s: %w", reference, err)
	}

	// Get image digest and size
	digest, err := image.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	size, err := image.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get image size: %w", err)
	}

	return &PushResult{
		Reference: reference,
		Digest:    digest.String(),
		Size:      size,
		PushedAt:  startTime,
	}, nil
}

// PullImage pulls an image from the OCI registry
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - reference: The image reference to pull (e.g., "registry.io/user/repo:tag")
//
// Returns:
//   - v1.Image: The pulled image
//   - *PullResult: Information about the pulled image
//   - error: An error if the pull operation fails
//
// Example:
//
//	image, result, err := client.PullImage(ctx, "docker.io/library/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Pulled image with digest: %s\n", result.Digest)
func (c *Client) PullImage(ctx context.Context, reference string) (v1.Image, *PullResult, error) {
	if reference == "" {
		return nil, nil, fmt.Errorf("reference cannot be empty")
	}

	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}

	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))

	// Pull the image
	startTime := time.Now()
	image, err := remote.Image(ref, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pull image from %s: %w", reference, err)
	}

	// Get image digest and size
	digest, err := image.Digest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	size, err := image.Size()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image size: %w", err)
	}

	result := &PullResult{
		Reference: reference,
		Digest:    digest.String(),
		Size:      size,
		PulledAt:  startTime,
	}

	return image, result, nil
}

// GetImageInfo retrieves detailed information about an image without pulling it
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - reference: The image reference to inspect
//
// Returns:
//   - *ImageInfo: Detailed information about the image
//   - error: An error if the operation fails
//
// Example:
//
//	info, err := client.GetImageInfo(ctx, "docker.io/library/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Image size: %d bytes\n", info.Size)
func (c *Client) GetImageInfo(ctx context.Context, reference string) (*ImageInfo, error) {
	if reference == "" {
		return nil, fmt.Errorf("reference cannot be empty")
	}

	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}

	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))

	// Get image descriptor
	desc, err := remote.Head(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get image descriptor for %s: %w", reference, err)
	}

	// Get manifest to extract more details
	manifest, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest for %s: %w", reference, err)
	}
	// Parse manifest to get additional information
	img, err := manifest.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to parse image from manifest: %w", err)
	}
	// Get config file for architecture and OS info
	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file: %w", err)
	}
	// Get creation time from config
	var createdAt time.Time
	if configFile.Created.Time != (time.Time{}) {
		createdAt = configFile.Created.Time
	}

	// Extract labels from config
	labels := make(map[string]string)
	if configFile.Config.Labels != nil {
		labels = configFile.Config.Labels
	}
	return &ImageInfo{
		Reference:    reference,
		Digest:       desc.Digest.String(),
		MediaType:    string(desc.MediaType),
		Size:         desc.Size,
		CreatedAt:    createdAt,
		Labels:       labels,
		Architecture: configFile.Architecture,
		OS:           configFile.OS,
	}, nil
}

// ListTags lists all tags for a given repository
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - repository: The repository name (e.g., "library/alpine", "myuser/myapp")
//
// Returns:
//   - []string: List of tags in the repository
//   - error: An error if the operation fails
//
// Example:
//
//	tags, err := client.ListTags(ctx, "library/alpine")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, tag := range tags {
//	    fmt.Printf("Tag: %s\n", tag)
//	}
func (c *Client) ListTags(ctx context.Context, repository string) ([]string, error) {
	if repository == "" {
		return nil, fmt.Errorf("repository cannot be empty")
	}
	// Parse repository reference
	repo, err := name.NewRepository(fmt.Sprintf("%s/%s", c.config.Registry, repository))
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository %s: %w", repository, err)
	}
	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	// List tags
	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for repository %s: %w", repository, err)
	}
	return tags, nil
}

// DeleteImage deletes an image from the registry by reference
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - reference: The image reference to delete
//
// Returns:
//   - error: An error if the delete operation fails
//
// Example:
//
//	err := client.DeleteImage(ctx, "docker.io/myuser/myapp:v1.0.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Image deleted successfully")
func (c *Client) DeleteImage(ctx context.Context, reference string) error {
	if reference == "" {
		return fmt.Errorf("reference cannot be empty")
	}
	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}
	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	// Delete the image
	if err := remote.Delete(ref, opts...); err != nil {
		return fmt.Errorf("failed to delete image %s: %w", reference, err)
	}
	return nil
}

// ImageExists checks if an image exists in the registry without pulling it
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - reference: The image reference to check
//
// Returns:
//   - bool: true if image exists, false otherwise
//   - error: An error if the operation fails (excluding not found errors)
//
// Example:
//
//	exists, err := client.ImageExists(ctx, "docker.io/library/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if exists {
//	    fmt.Println("Image exists")
//	}
func (c *Client) ImageExists(ctx context.Context, reference string) (bool, error) {
	if reference == "" {
		return false, fmt.Errorf("reference cannot be empty")
	}
	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return false, fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}
	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	// Try to get image descriptor
	_, err = remote.Head(ref, opts...)
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "404") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "MANIFEST_UNKNOWN") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check image existence: %w", err)
	}
	return true, nil
}

// CopyImage copies an image from one reference to another
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - srcReference: Source image reference
//   - dstReference: Destination image reference
//
// Returns:
//   - *PushResult: Information about the copied image
//   - error: An error if the copy operation fails
//
// Example:
//
//	result, err := client.CopyImage(ctx,
//	    "docker.io/library/alpine:latest",
//	    "myregistry.io/myuser/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Copied image with digest: %s\n", result.Digest)
func (c *Client) CopyImage(ctx context.Context, srcReference, dstReference string) (*PushResult, error) {
	if srcReference == "" {
		return nil, fmt.Errorf("source reference cannot be empty")
	}
	if dstReference == "" {
		return nil, fmt.Errorf("destination reference cannot be empty")
	}
	// Pull the source image
	image, _, err := c.PullImage(ctx, srcReference)
	if err != nil {
		return nil, fmt.Errorf("failed to pull source image: %w", err)
	}
	// Push to destination
	result, err := c.PushImage(ctx, image, dstReference)
	if err != nil {
		return nil, fmt.Errorf("failed to push to destination: %w", err)
	}
	return result, nil
}

// GetManifest retrieves the raw manifest for an image
//
// Parameters:
//   - ctx: Context for the operation (required)
//   - reference: The image reference
//
// Returns:
//   - []byte: Raw manifest bytes
//   - string: Media type of the manifest
//   - error: An error if the operation fails
//
// Example:
//
//	manifest, mediaType, err := client.GetManifest(ctx, "docker.io/library/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Manifest media type: %s\n", mediaType)
func (c *Client) GetManifest(ctx context.Context, reference string) ([]byte, string, error) {
	if reference == "" {
		return nil, "", fmt.Errorf("reference cannot be empty")
	}
	// Parse the reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse reference %s: %w", reference, err)
	}
	// Setup remote options with context
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	// Get manifest
	manifest, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get manifest for %s: %w", reference, err)
	}
	return manifest.Manifest, string(manifest.MediaType), nil
}

// ValidateReference validates if a reference string is properly formatted
//
// Parameters:
//   - reference: The reference string to validate
//
// Returns:
//   - bool: true if reference is valid, false otherwise
//   - error: An error describing what's wrong with the reference
//
// Example:
//
//	valid, err := oci.ValidateReference("docker.io/library/alpine:latest")
//	if !valid {
//	    fmt.Printf("Invalid reference: %v\n", err)
//	}
func ValidateReference(reference string) (bool, error) {
	if reference == "" {
		return false, fmt.Errorf("reference cannot be empty")
	}
	_, err := name.ParseReference(reference)
	if err != nil {
		return false, fmt.Errorf("invalid reference format: %w", err)
	}
	return true, nil
}

// ParseReference parses a reference string and returns its components
//
// Parameters:
//   - reference: The reference string to parse
//
// Returns:
//   - registry: The registry hostname
//   - repository: The repository name
//   - tag: The tag (if present)
//   - digest: The digest (if present)
//   - error: An error if parsing fails
//
// Example:
//
//	registry, repo, tag, digest, err := oci.ParseReference("docker.io/library/alpine:latest")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Registry: %s, Repo: %s, Tag: %s\n", registry, repo, tag)
func ParseReference(reference string) (registry, repository, tag, digest string, err error) {
	ref, err := name.ParseReference(reference)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse reference: %w", err)
	}
	registry = ref.Context().RegistryStr()
	repository = ref.Context().RepositoryStr()
	// Check if it's a tag or digest reference
	if tagRef, ok := ref.(name.Tag); ok {
		tag = tagRef.TagStr()
	}
	if digestRef, ok := ref.(name.Digest); ok {
		digest = digestRef.DigestStr()
	}
	return registry, repository, tag, digest, nil
}
