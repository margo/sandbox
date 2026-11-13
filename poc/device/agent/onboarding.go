// auth/onboarding.go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/poc/device/agent/types"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/shared-lib/mis/parser"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

// onboarding.go
type DeviceClientSettings struct {
	wfmEndpointsForClient    []string
	log                      *zap.SugaredLogger
	apiClient                wfm.SBIAPIClientInterface
	db                       database.DatabaseIfc
	supportedDeploymentTypes []sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes
	supportedRuntimes        []sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimes
	miaf                     types.MIAFConfig
	parsedMiaf               *parser.ParsedMIAFConfig
}

type Option = func(auth *DeviceClientSettings)

func WithEnableComposeDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.supportedDeploymentTypes = append(
			auth.supportedDeploymentTypes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose,
		)

		auth.supportedRuntimes = append(
			auth.supportedRuntimes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimesOci,
		)
	}
}

func WithMIAFConfig(cfg types.MIAFConfig) Option {
	return func(auth *DeviceClientSettings) {
		auth.miaf = cfg
	}
}

// WithParsedMIAFConfig sets the pre-parsed MIAF configuration on DeviceClientSettings.
func WithParsedMIAFConfig(cfg *parser.ParsedMIAFConfig) Option {
	return func(auth *DeviceClientSettings) {
		auth.parsedMiaf = cfg
	}
}

func WithEnableHelmDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.supportedDeploymentTypes = append(
			auth.supportedDeploymentTypes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesHelm,
		)

		auth.supportedRuntimes = append(
			auth.supportedRuntimes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimesOci,
		)
	}
}

func NewDeviceSettings(
	client wfm.SBIAPIClientInterface,
	db database.DatabaseIfc,
	log *zap.SugaredLogger,
	opts ...Option,
) (*DeviceClientSettings, error) {
	existingRecord, err := db.GetDeviceSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get device settings from database, %s", err.Error())
	}

	var supportedDeploymentTypes []sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes
	var supportedRuntimes []sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimes

	if existingRecord != nil {
		supportedDeploymentTypes = existingRecord.SupportedDeploymentTypes
		supportedRuntimes = existingRecord.SupportedRuntimes
	}

	settings := &DeviceClientSettings{
		apiClient:                client,
		log:                      log,
		db:                       db,
		supportedDeploymentTypes: supportedDeploymentTypes,
		supportedRuntimes:        supportedRuntimes,
	}

	for _, opt := range opts {
		opt(settings)
	}

	newDeviceRecord := database.DeviceSettingsRecord{}
	if existingRecord != nil {
		newDeviceRecord = *existingRecord
	}
	newDeviceRecord.MIAF = settings.miaf
	if settings.parsedMiaf != nil {
		newDeviceRecord.ParsedMIAF = *settings.parsedMiaf
	}
	newDeviceRecord.SupportedDeploymentTypes = settings.supportedDeploymentTypes
	newDeviceRecord.SupportedRuntimes = settings.supportedRuntimes

	if err := db.SetDeviceSettings(newDeviceRecord); err != nil {
		return nil, err
	}

	return settings, nil
}

func (da *DeviceClientSettings) ReportCapabilities(
	ctx context.Context,
	capabilities sbi.DeviceCapabilitiesManifest,
) error {
	da.log.Infow("Starting capabilities reporting",
		"deviceClientId", capabilities.Properties.Id,
		"cpu", summarizeCapabilitiesCPU(capabilities),
		"cache", summarizeCapabilitiesCache(capabilities),
		"memory", summarizeCapabilitiesMemory(capabilities))
	err := da.apiClient.ReportCapabilities(ctx, capabilities.Properties.Id, capabilities)
	if err != nil {
		if pd, ok := sbi.AsProblemDetail(err); ok {
			da.log.Errorw(
				"WFM returned problem detail on capabilities report",
				"deviceId", capabilities.Properties.Id,
				"type", pd.Type,
				"status", pd.Status,
				"detail", pd.Detail,
				"retryable", pd.IsRetryable(),
			)
			return fmt.Errorf("WFM rejected capabilities [%d] %s: %w", pd.Status, pd.Title, err)
		}
		return fmt.Errorf("failed to report capabilities: %w", err)
	}
	da.log.Infow("Capabilities reported successfully", "deviceClientId", capabilities.Properties.Id)
	return nil
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func summarizeCapabilitiesCPU(capabilities sbi.DeviceCapabilitiesManifest) string {
	cpus := capabilities.Properties.Cpus
	if cpus == nil || len(*cpus) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(*cpus))
	for cpuIndex, cpu := range *cpus {
		architecture := "<nil>"
		if cpu.Architecture != nil {
			architecture = string(*cpu.Architecture)
		}

		cpuClass := "<nil>"
		if cpu.Class != nil {
			cpuClass = string(*cpu.Class)
		}

		cpuType := "<nil>"
		if cpu.Type != nil {
			cpuType = string(*cpu.Type)
		}

		baseMHz := "<nil>"
		maxMHz := "<nil>"
		if cpu.Frequency != nil {
			if cpu.Frequency.BaseMHz != nil {
				baseMHz = fmt.Sprintf("%g", *cpu.Frequency.BaseMHz)
			}
			if cpu.Frequency.MaxMHz != nil {
				maxMHz = fmt.Sprintf("%g", *cpu.Frequency.MaxMHz)
			}
		}

		parts = append(parts, fmt.Sprintf("cpu[%d]={cores=%g, class=%s, frequency={baseMHz=%s, maxMHz=%s}, type=%s, architecture=%s}",
			cpuIndex, cpu.Cores, cpuClass, baseMHz, maxMHz, cpuType, architecture))
	}

	return strings.Join(parts, "; ")
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func summarizeCapabilitiesCache(capabilities sbi.DeviceCapabilitiesManifest) string {
	caches := capabilities.Properties.Cache
	if caches == nil || len(*caches) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(*caches))
	for cacheIndex, cache := range *caches {
		size := cache.Size
		if size == "" {
			size = "<empty>"
		}

		allocationTypes := "none"
		if len(cache.AllocationTypes) > 0 {
			allocations := make([]string, 0, len(cache.AllocationTypes))
			for _, allocationType := range cache.AllocationTypes {
				allocations = append(allocations, string(allocationType))
			}
			allocationTypes = strings.Join(allocations, "|")
		}

		parts = append(parts, fmt.Sprintf("cache[%d]={level=%s, allocationTypes=[%s], size=%s}",
			cacheIndex, cache.Level, allocationTypes, size))
	}

	return strings.Join(parts, "; ")
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func summarizeCapabilitiesMemory(capabilities sbi.DeviceCapabilitiesManifest) string {
	memory := capabilities.Properties.Memory
	if memory == nil {
		return "none"
	}
	bwTypes := "none"
	if memory.BandwidthAllocationTypes != nil && len(*memory.BandwidthAllocationTypes) > 0 {
		parts := make([]string, 0, len(*memory.BandwidthAllocationTypes))
		for _, bwType := range *memory.BandwidthAllocationTypes {
			parts = append(parts, string(bwType))
		}
		bwTypes = strings.Join(parts, ",")
	}
	return fmt.Sprintf("memory={size=%s, bandwidthAllocationTypes=%s}", memory.Size, bwTypes)
}

func (da *DeviceClientSettings) IsOnboarded() (bool, error) {
	_, isOnboarded, err := da.db.IsDeviceOnboarded()
	return isOnboarded, err
}
