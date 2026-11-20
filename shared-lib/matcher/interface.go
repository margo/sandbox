package matcher

import (
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// TargetProfile associates a workload target identifier with its reported device profile.
type TargetProfile struct {
	TargetID string
	Profile  sbi.Profile
}

// MatcherIface defines the contract for matching workload target profiles against
// deployment technology requirements and application match expressions.
type MatcherIface interface {
	// IsTargetEligible evaluates a single device profile against:
	// 1. Mandatory deployment technology key (e.g. "margo.org/deployment/helm")
	// 2. Application placementConstraints groups (OR across groups, AND within each group's matchExpressions).
	// If placementConstraints is empty or nil, only deploymentKey is evaluated.
	// Returns:
	//   - eligible: true if the target profile satisfies the requirements
	//   - reason: explanation of failure if not eligible
	//   - err: non-nil only on fatal evaluation errors (e.g., malformed JSON Pointer)
	IsTargetEligible(
		profile *sbi.Profile,
		placementConstraints []sbi.PlacementConstraints,
	) (bool, string, error)
}
