package matcher

import (
	"fmt"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// New creates and returns a new MatcherIface instance.
func New() MatcherIface {
	return &deviceMatcher{}
}

type deviceMatcher struct{}

// IsTargetEligible evaluates a single device profile against:
// 1. Mandatory deployment technology key (e.g., "margo.org/deployment/helm")
// 2. Application characteristics groups (OR across groups, AND within each group's matchExpressions).
func (m *deviceMatcher) IsTargetEligible(
	profile *clModels.Profile,
	deploymentKey string,
	characteristics []clModels.Characteristics,
) (bool, string, error) {
	if profile == nil {
		return false, "device profile is nil", nil
	}

	// 1. Mandatory deployment technology check
	if deploymentKey != "" {
		if !hasCharacteristicKey(profile, deploymentKey) {
			return false, fmt.Sprintf("device does not support deployment technology %q", deploymentKey), nil
		}
	}

	// 2. If no characteristics expressions defined, device is eligible
	if len(characteristics) == 0 {
		return true, "", nil
	}

	// 3. Evaluate characteristics groups (OR logic across groups, AND logic within each group)
	var lastReason string
	for _, group := range characteristics {
		allPassed := true
		for _, expr := range group.MatchExpressions {
			ok, reason, err := m.evaluateSingleMatchExpression(profile, &expr)
			if err != nil {
				return false, "", err
			}
			if !ok {
				allPassed = false
				lastReason = reason
				break
			}
		}
		if allPassed {
			return true, "", nil
		}
	}

	return false, lastReason, nil
}

// hasCharacteristicKey checks if the profile contains a characteristic with the given key.
func hasCharacteristicKey(profile *clModels.Profile, key string) bool {
	for _, c := range profile.Characteristics {
		if c.Key == key {
			return true
		}
	}
	return false
}

// findCharacteristic returns the characteristic with the given key, or nil if not found.
func findCharacteristic(profile *clModels.Profile, key string) *clModels.Characteristic {
	for i := range profile.Characteristics {
		if profile.Characteristics[i].Key == key {
			return &profile.Characteristics[i]
		}
	}
	return nil
}

// evaluateSingleMatchExpression evaluates one MatchExpression against a device profile.
func (m *deviceMatcher) evaluateSingleMatchExpression(
	profile *clModels.Profile,
	me *clModels.MatchExpression,
) (bool, string, error) {
	char := findCharacteristic(profile, me.Key)
	charFound := (char != nil)

	// When property is omitted (nil or empty), test characteristic key presence
	if me.Property == nil || *me.Property == "" {
		propDesc := fmt.Sprintf("characteristic key %q", me.Key)
		switch me.Operator {
		case clModels.MatchExpressionOperatorExists:
			ok, reason := evaluateExists(charFound, me.Values, propDesc)
			return ok, reason, nil
		case clModels.MatchExpressionOperatorDoesNotExist:
			ok, reason := evaluateDoesNotExist(charFound, me.Values, propDesc)
			return ok, reason, nil
		default:
			return false, fmt.Sprintf("property pointer is required for operator %s (key %q)", me.Operator, me.Key), nil
		}
	}

	pointer := *me.Property
	propDesc := fmt.Sprintf("characteristic %q property %q", me.Key, pointer)

	// If characteristic key is absent from the profile
	if !charFound {
		switch me.Operator {
		case clModels.MatchExpressionOperatorDoesNotExist:
			// Characteristic key is absent, so property is also absent
			ok, reason := evaluateDoesNotExist(false, me.Values, propDesc)
			return ok, reason, nil
		default:
			return false, fmt.Sprintf("characteristic key %q not found in device profile", me.Key), nil
		}
	}

	// Normalize properties bag for uniform JSON traversal
	normProps, err := normalizeJSON(char.Properties)
	if err != nil {
		return false, "", fmt.Errorf("failed to process properties for characteristic %q: %w", me.Key, err)
	}

	// Resolve RFC 6901 JSON Pointer relative to properties
	resolved, exists, err := resolvePointer(normProps, pointer)
	if err != nil {
		return false, "", fmt.Errorf("invalid property pointer %q for characteristic %q: %w", pointer, me.Key, err)
	}

	switch me.Operator {
	case clModels.MatchExpressionOperatorExists:
		ok, reason := evaluateExists(exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorDoesNotExist:
		ok, reason := evaluateDoesNotExist(exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorIn:
		ok, reason := evaluateIn(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorNotIn:
		ok, reason := evaluateNotIn(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorGt:
		ok, reason := evaluateGt(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorGtEq:
		ok, reason := evaluateGtEq(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorLt:
		ok, reason := evaluateLt(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorLtEq:
		ok, reason := evaluateLtEq(resolved, exists, me.Values, propDesc)
		return ok, reason, nil
	case clModels.MatchExpressionOperatorContainsAll:
		return evaluateContainsAll(resolved, exists, me, propDesc)
	case clModels.MatchExpressionOperatorContainsAny:
		return evaluateContainsAny(resolved, exists, me, propDesc)
	default:
		return false, "", fmt.Errorf("unknown or unsupported operator %s", me.Operator)
	}
}
