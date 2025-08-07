package oci

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Config holds OCI registry configuration and authentication details
type Config struct {
	Registry   string        // Registry URL (e.g., "gcr.io", "docker.io", "localhost:5000")
	Username   string        // Registry username
	Password   string        // Registry password/token
	Insecure   bool          // Allow insecure connections (HTTP instead of HTTPS)
	Timeout    time.Duration // Request timeout (default: 30 seconds)
	UserAgent  string        // Custom user agent string
	CABundle   []byte        // CA bundle (PEM encoded) for custom certificates
	ClientCert []byte        // Client certificate (PEM encoded)
	ClientKey  []byte        // Private key (PEM encoded) for client certificate
}

// Client provides operations for interacting with OCI registries
type Client struct {
	config     *Config
	auth       authn.Authenticator
	remoteOpts []remote.Option
}

// NewClient creates a new OCI registry client with the provided configuration
//
// Parameters:
//   - config: OCI registry configuration (required, cannot be nil)
//
// Returns:
//   - *Client: A new OCI client instance
//   - error: An error if client creation fails
//
// Example:
//
//	config := &oci.Config{
//	    Registry: "docker.io",
//	    Username: "myuser",
//	    Password: "mytoken",
//	    Timeout:  30 * time.Second,
//	}
//	client, err := oci.NewClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	// Set default timeout if not provided
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	// Set default user agent if not provided
	if config.UserAgent == "" {
		config.UserAgent = "oci-client/1.0"
	}
	client := &Client{
		config: config,
	}
	// Setup authentication
	if err := client.setupAuth(); err != nil {
		return nil, fmt.Errorf("failed to setup authentication: %w", err)
	}
	// Setup remote options
	if err := client.setupRemoteOptions(); err != nil {
		return nil, fmt.Errorf("failed to setup remote options: %w", err)
	}
	return client, nil
}

// setupAuth configures authentication for the OCI client
func (c *Client) setupAuth() error {
	// Default to anonymous authentication
	c.auth = authn.Anonymous
	// Setup basic authentication if credentials are provided
	if c.config.Username != "" && c.config.Password != "" {
		c.auth = &authn.Basic{
			Username: c.config.Username,
			Password: c.config.Password,
		}
	}
	return nil
}

// setupRemoteOptions configures remote options for registry operations
func (c *Client) setupRemoteOptions() error {
	c.remoteOpts = []remote.Option{
		remote.WithAuth(c.auth),
		remote.WithUserAgent(c.config.UserAgent),
	}
	// Setup custom transport if needed
	transport := &http.Transport{
		ResponseHeaderTimeout: c.config.Timeout,
	}
	// Configure TLS settings
	if c.config.Insecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Add custom CA bundle if provided
	if len(c.config.CABundle) > 0 {
		// TODO: Implement custom CA bundle loading
		// This would require creating a custom cert pool
	}
	// Add client certificate if provided
	if len(c.config.ClientCert) > 0 && len(c.config.ClientKey) > 0 {
		// TODO: Implement client certificate loading
		// This would require parsing the cert and key
	}
	c.remoteOpts = append(c.remoteOpts, remote.WithTransport(transport))

	return nil
}

// Ping checks if the registry is accessible and returns basic information
//
// Parameters:
//   - ctx: Context for the operation (required)
//
// Returns:
//   - bool: true if registry is accessible, false otherwise
//   - error: An error if the ping operation fails
//
// Example:
//
//	accessible, err := client.Ping(context.Background())
//	if err != nil {
//	    log.Printf("Registry ping failed: %v", err)
//	}
//	if accessible {
//	    fmt.Println("Registry is accessible")
//	}
func (c *Client) Ping(ctx context.Context) (bool, error) {
	// Create a dummy reference to test connectivity
	registryHost := c.config.Registry
	if !strings.Contains(registryHost, "/") {
		registryHost = registryHost + "/library/hello-world"
	}
	ref, err := name.ParseReference(registryHost + ":latest")
	if err != nil {
		return false, fmt.Errorf("failed to parse reference for ping: %w", err)
	}

	// Try to get the manifest (this will test authentication and connectivity)
	opts := append(c.remoteOpts, remote.WithContext(ctx))
	_, err = remote.Head(ref, opts...)
	if err != nil {
		// If it's a 404, the registry is accessible but the image doesn't exist
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return true, nil
		}
		return false, fmt.Errorf("registry ping failed: %w", err)
	}

	return true, nil
}

// Close cleans up any resources used by the client
//
// This method should be called when the client is no longer needed
// to ensure proper cleanup of resources.
//
// Example:
//
//	defer client.Close()
func (c *Client) Close() error {
	// Currently no resources to clean up, but this provides
	// a hook for future implementations that might need cleanup
	return nil
}

// GetConfig returns a copy of the client configuration
//
// Returns:
//   - *Config: A copy of the client configuration
//
// Example:
//
//	config := client.GetConfig()
//	fmt.Printf("Registry: %s\n", config.Registry)
func (c *Client) GetConfig() *Config {
	// Return a copy to prevent external modification
	configCopy := *c.config
	return &configCopy
}
