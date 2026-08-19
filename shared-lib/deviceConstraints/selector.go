package deviceconstraints

import (
	"fmt"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type DeviceSelectorIface interface {
	SelectEligibleDevice(devices []*clModels.DeviceCapabilitiesManifest, checks *clModels.DeviceConstraints) ([]*clModels.DeviceCapabilitiesManifest, error)
	IsDeviceEligible(device *clModels.DeviceCapabilitiesManifest, checks *clModels.DeviceConstraints) (bool, string, error)
}

func NewDeviceSelector(logger *zap.SugaredLogger) DeviceSelectorIface {
	return &deviceSelectorImplementation{logger: logger}
}

type deviceSelectorImplementation struct {
	logger *zap.SugaredLogger
}

/*
SelectEligibleDevice evaluates the provided devices against the specified
constraints and returns all devices that satisfy every eligibility check.
An error is returned if a fatal error occurs during evaluation.
When a non-nil error is returned, the resulting device slice is guaranteed to be empty.
*/
func (ds *deviceSelectorImplementation) SelectEligibleDevice(
	devices []*clModels.DeviceCapabilitiesManifest,
	checks *clModels.DeviceConstraints) ([]*clModels.DeviceCapabilitiesManifest, error) {

	eligibleDevs := make([]*clModels.DeviceCapabilitiesManifest, 0)
	var gErr error
	for _, d := range devices {
		ok, reason, err := ds.IsDeviceEligible(d, checks)
		if err == nil && ok {
			eligibleDevs = append(eligibleDevs, d)
			continue
		}

		if err != nil {
			//log error message
			ds.logger.Errorw("failed to check for eligibility", "device id", d.Properties.Id, "err", err.Error())
			gErr = err
			break
		}

		if !ok {
			// log ineligbility
			ds.logger.Warnw("device is ineligible, moving on to next device", "device id", d.Properties.Id, "reason", reason)
			continue
		}
	}

	// In case of error, do not continue checking for other devices. Halt.
	// Error means some problem occurred while checking, not ineligibility.
	if gErr != nil {
		return nil, gErr
	}

	return eligibleDevs, nil
}

// IsDeviceEligible evaluates a single device against the provided constraints.
// It returns true if the device satisfies all eligibility checks. If the
// device is not eligible, it returns false along with a reason describing the
// first failed eligibility check. A non-nil error indicates a fatal failure
// during evaluation. When an error is returned, the eligibility result and
// reason are guaranteed to be their zero values (false, "").
func (ds *deviceSelectorImplementation) IsDeviceEligible(
	device *clModels.DeviceCapabilitiesManifest,
	checks *clModels.DeviceConstraints) (bool, string, error) {

	if checks == nil {
		return true, "", nil
	}

	dc := NewDeviceCapabilityChecker(*device)
	ok, reason, err := dc.CheckEligibility(checks.CapacityRequirements)
	if err != nil {
		// Handle error
		return false, "", fmt.Errorf("failed to check device eligibility, err: %s", err.Error())
	}

	if !ok {
		return false, fmt.Sprintf("not enough resources: capacity : %s", reason), nil
	}

	lse := NewLabelSelectorEngine(device.Labels)
	pse := NewPropertySelectorEngine(device)

	finalReason := ""
	// Out of all Eligibility rules, at least 1 needs to pass to consider that device.
	// Hence, we just need to find that particular rule.
	// Label Eligibility & Property Eligibility have AND relationship
	for _, v := range *checks.EligibilityRules {

		// continue checking labels
		if v.LabelSelector != nil {
			ok, reason := lse.Evaluate(v.LabelSelector)
			if !ok {
				finalReason = reason
				continue
			}
		}

		if v.PropertySelector != nil {
			ok, reason := pse.Evaluate(v.PropertySelector)
			if !ok {
				finalReason = reason
				continue
			}
		}

		return true, "", nil
	}

	return false, finalReason, nil
}
