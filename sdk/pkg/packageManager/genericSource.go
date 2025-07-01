package packageManager

import (
	"context"

	"github.com/margo/dev-repo/sdk/pkg/models"
)

// Source types with embedded behavior
type PackageSource[T any] interface {
	Load(ctx context.Context) (*models.ApplicationPackage, error)
	Validate() error
	GetMetadata() T
}
