package pkg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

func ParseAppDeploymentFromBase64(base64Yaml string) (*sbi.AppDeploymentManifest, error) {
	decodedYaml, err := base64.StdEncoding.DecodeString(base64Yaml)
	if err != nil {
		// da.log.Errorw("Failed to decode base64 AppDeploymentYAML", "appId", state.AppId, "error",
		// err)
		return nil, fmt.Errorf(
			"failed to decode the app deployment yaml from its base64 format, err: %w",
			err,
		)
	}

	var appDeployment sbi.AppDeploymentManifest
	if err := json.Unmarshal(decodedYaml, &appDeployment); err != nil {
		// da.log.Errorw("Failed to unmarshal JSON AppDeployment", "appId", state.AppId, "error",
		// err)
		return nil, fmt.Errorf(
			"failed to parse the app deployment object from the yaml, err: %w",
			err,
		)
	}

	return &appDeployment, nil
}

// ConvertAppDeploymentParamsToValues converts AppDeploymentParams to a map[string]interface{}
// that can be used for Helm chart value overrides
func ConvertAppDeploymentParamsToValues(
	params sbi.AppDeploymentParams,
	componentName string,
) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	for paramName, paramValue := range params {
		// Check if this parameter applies to the specified component
		if !parameterAppliesToComponent(paramValue, componentName) {
			continue
		}

		// For each target that matches the component, set the value
		for _, target := range paramValue.Targets {
			if containsComponent(target.Components, componentName) {
				err := setNestedValue(values, target.Pointer, paramValue.Value)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to set value for parameter %s: %w",
						paramName,
						err,
					)
				}
			}
		}
	}

	return values, nil
}

// ConvertAllAppDeploymentParamsToValues converts all parameters to a component-wise map
func ConvertAllAppDeploymentParamsToValues(
	params sbi.AppDeploymentParams,
) (map[string]map[string]interface{}, error) {
	/*componentNameVsValues*/
	componentValues := make(map[string]map[string]interface{})

	// Collect all unique component names
	components := make(map[string]bool)
	for _, paramValue := range params {
		for _, target := range paramValue.Targets {
			for _, comp := range target.Components {
				components[comp] = true
			}
		}
	}

	// Convert parameters for each component
	for componentName := range components {
		values, err := ConvertAppDeploymentParamsToValues(params, componentName)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			componentValues[componentName] = values
		}
	}

	return componentValues, nil
}

// ConvertToFlatMap converts AppDeploymentParams to a flat map[string]interface{}
// where keys are the parameter names and values are the parameter values
func ConvertToFlatMap(params sbi.AppDeploymentParams) map[string]interface{} {
	flatMap := make(map[string]interface{})

	for paramName, paramValue := range params {
		flatMap[paramName] = paramValue.Value
	}

	return flatMap
}

// Helper function to check if a parameter applies to a specific component
func parameterAppliesToComponent(paramValue sbi.AppParameterValue, componentName string) bool {
	for _, target := range paramValue.Targets {
		if containsComponent(target.Components, componentName) {
			return true
		}
	}
	return false
}

// Helper function to check if a component list contains a specific component
func containsComponent(components []string, componentName string) bool {
	for _, comp := range components {
		if comp == componentName {
			return true
		}
	}
	return false
}

// Helper function to set nested values in a map using dot notation
func setNestedValue(values map[string]interface{}, pointer string, value interface{}) error {
	keys := strings.Split(pointer, ".")
	current := values

	// Navigate to the parent of the final key
	for i, key := range keys[:len(keys)-1] {
		if current[key] == nil {
			current[key] = make(map[string]interface{})
		}

		// Type assertion to ensure we have a map
		if nested, ok := current[key].(map[string]interface{}); ok {
			current = nested
		} else {
			return fmt.Errorf("conflict at key path %s: expected map but found %T",
				strings.Join(keys[:i+1], "."), current[key])
		}
	}

	// Set the final value
	finalKey := keys[len(keys)-1]
	current[finalKey] = value // convertStringValue(value)

	return nil
}

// Helper function to convert string values to appropriate types
func convertStringValue(value string) interface{} {
	// Try to convert to common types
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		// Try to parse as number
		if strings.Contains(value, ".") {
			if f, err := parseFloat(value); err == nil {
				return f
			}
		} else {
			if i, err := parseInt(value); err == nil {
				return i
			}
		}
		// Return as string if no conversion possible
		return value
	}
}

// Helper functions for number parsing
func parseFloat(s string) (float64, error) {
	return 0.0, fmt.Errorf("not implemented")
}

func parseInt(s string) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}
