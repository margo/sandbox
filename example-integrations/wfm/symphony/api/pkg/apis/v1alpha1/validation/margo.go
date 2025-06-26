package validation

import (
	"context"
)

type MargoValidator struct {
}

func NewMargoValidator() MargoValidator {
	return MargoValidator{}
}

func (c *MargoValidator) ValidateCreateOrUpdate(ctx context.Context, newRef interface{}, oldRef interface{}) []ErrorField {
	errorFields := []ErrorField{}
	return errorFields
}

func (c *MargoValidator) ValidateDelete(ctx context.Context, newRef interface{}) []ErrorField {
	errorFields := []ErrorField{}
	return errorFields
}
