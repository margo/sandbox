package packageManager

import (
	"context"
	"io/fs"
	"time"

	"github.com/margo/sandbox/non-standard/pkg/models"
)

// SourceType defines where packages come from
type SourceType string

const (
	SourceDirectory SourceType = "directory"
	SourceOCI       SourceType = "oci"
)

// Loader loads packages from a specific source type
type Loader interface {
	Load(
		ctx context.Context,
		source string,
		opts *LoadOptions,
	) (string /*path*/, *models.AppPkg, error)
	Type() SourceType
}

// LoadOptions configures loading behavior
type LoadOptions struct {
	// OCI-specific
	Tag      string
	Username string
	Token    string
	Insecure bool
	IsHTTP   bool

	// Common
	Validate bool
	Timeout  time.Duration
}

// Config holds package manager configuration
type Config struct {
	TempDir          string
	EnableValidation bool
	MaxPackageSize   int64
	DirPermissions   fs.FileMode
	FilePermissions  fs.FileMode
}

// Default values
const (
	DefaultDirPermissions  = 0750
	DefaultFilePermissions = 0600
	DefaultTimeout         = 30 * time.Second
)
