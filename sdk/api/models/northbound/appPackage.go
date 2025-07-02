package northbound

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/margo/dev-repo/sdk/pkg/models"
)

// Common response wrapper for consistent API responses
type APIResponse[T any] struct {
	Success   bool   `json:"success"`
	Data      T      `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Timestamp string `json:"timestamp"`
}

// AppPackageOnboardingReq represents the request to onboard an application
type AppPackageOnboardingReq struct {
	// Git repository configuration
	GitURL         string  `json:"gitUrl" validate:"required,url"`
	GitBranch      *string `json:"gitBranch,omitempty"`
	GitUserName    *string `json:"gitUsername,omitempty"`
	GitAccessToken *string `json:"gitAccessToken,omitempty"`
}

// Validate validates the onboarding request
func (r *AppPackageOnboardingReq) Validate() error {
	if r.GitURL == "" {
		return fmt.Errorf("gitUrl is required")
	}

	if _, err := url.Parse(r.GitURL); err != nil {
		return fmt.Errorf("invalid gitUrl format: %w", err)
	}

	return nil
}

// ParseAppPackageOnboardingReqFromBytes parses JSON bytes into AppPackageOnboardingReq
func ParseAppPackageOnboardingReqFromBytes(data []byte) (*AppPackageOnboardingReq, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty request data")
	}

	var req AppPackageOnboardingReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &req, nil
}

// ValidationError represents validation errors
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// AppPackageOnboardingResp represents the response from app onboarding
type AppPackageOnboardingResp struct {
	AppId     string                                  `json:"appId"`
	Name      string                                  `json:"name"`
	Version   string                                  `json:"version"`
	State     models.ApplicationPackageOperationState `json:"state"`
	GitURL    string                                  `json:"gitUrl"`
	GitBranch string                                  `json:"gitBranch,omitempty"`
	CreatedAt time.Time                               `json:"createdAt"`
	UpdatedAt time.Time                               `json:"updatedAt"`
}

// ListAppPackagesReq represents the request to list applications
type ListAppPackagesReq struct {
	PaginationRequest
	// Filters
	Name  *string                                 `json:"name,omitempty" query:"name"`
	State models.ApplicationPackageOperationState `json:"state"`
	// Date filters
	CreatedAfter  *time.Time `json:"createdAfter,omitempty" query:"createdAfter"`
	CreatedBefore *time.Time `json:"createdBefore,omitempty" query:"createdBefore"`
}

// SetDefaults sets default values for pagination and sorting
func (r *ListAppPackagesReq) SetDefaults() {
	if r.Page == 0 {
		r.Page = 1
	}
	if r.PageSize == 0 {
		r.PageSize = 20
	}
}

// AppPackageSummary represents a summary of an application
type AppPackageSummary struct {
	AppId     string                                  `json:"appId"`
	Name      string                                  `json:"name"`
	Version   string                                  `json:"version"`
	Status    models.ApplicationPackageOperationState `json:"status"`
	GitURL    string                                  `json:"gitUrl"`
	GitBranch string                                  `json:"gitBranch,omitempty"`
	CreatedAt time.Time                               `json:"createdAt"`
	UpdatedAt time.Time                               `json:"updatedAt"`
}

// ListAppPackagesResp represents the response from listing applications
type ListAppPackagesResp struct {
	Apps       []AppPackageSummary `json:"appPackages"`
	Pagination PaginationResponse  `json:"pagination"`
}

// DeleteAppPackageReq represents the request to delete an application
type DeleteAppPackageReq struct {
	AppId         string `json:"appId" path:"appId" validate:"required"`
	Force         bool   `json:"force,omitempty" query:"force"`
	DeleteData    bool   `json:"deleteData,omitempty" query:"deleteData"`
	ConfirmDelete string `json:"confirmDelete,omitempty" validate:"omitempty,eq=DELETE"`
}

// Validate validates the delete request
func (r *DeleteAppPackageReq) Validate() error {
	if r.AppId == "" {
		return fmt.Errorf("appId is required")
	}

	if r.Force && r.ConfirmDelete != "DELETE" {
		return fmt.Errorf("confirmDelete must be 'DELETE' when force is true")
	}

	return nil
}

// DeleteAppPackageResp represents the response from deleting an application
type DeleteAppPackageResp struct {
	AppId string                                  `json:"appId"`
	State models.ApplicationPackageOperationState `json:"state"`
}

// Helper functions for creating responses
func NewSuccessResponse[T any](data T, requestID string) APIResponse[T] {
	return APIResponse[T]{
		Success:   true,
		Data:      data,
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func NewErrorResponse[T any](err string, requestID string) APIResponse[T] {
	return APIResponse[T]{
		Success:   false,
		Error:     err,
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
