package packageManager

import (
	"context"
	"fmt"

	"github.com/margo/sandbox/non-standard/generatedCode/wfm/nbi"
	"github.com/margo/sandbox/non-standard/pkg/models"
)

// Validator validates packages and descriptions
type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(ctx context.Context, pkg *models.AppPkg) error {
	if err := v.ValidateDescription(ctx, pkg.Description); err != nil {
		return err
	}

	if err := v.ValidateResources(ctx, pkg.Resources); err != nil {
		return err
	}

	return nil
}

func (v *Validator) ValidateDescription(ctx context.Context, desc *nbi.AppDescription) error {
	if desc == nil {
		return fmt.Errorf("description is nil")
	}

	if desc.ApiVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}


	if desc.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	if desc.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}

	return nil
}

func (v *Validator) ValidateResources(ctx context.Context, resources map[string][]byte) error {
	// Add resource validation logic here
	// e.g., check file sizes, types, etc.
	return nil
}
