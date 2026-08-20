package property

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/margo/sandbox/shared-lib/constraints/common"
	"github.com/margo/sandbox/shared-lib/pointers"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// ---------------------------------------------------------------------------
// Helpers — build a minimal but valid DeviceCapabilitiesManifest
// ---------------------------------------------------------------------------

func deviceWithVendor(vendor string) *clModels.DeviceCapabilitiesManifest {
	return &clModels.DeviceCapabilitiesManifest{
		ApiVersion: "device.margo.org/v1alpha1",
		Kind:       clModels.DeviceCapabilitiesManifestKindDeviceCapabilitiesManifest,
		Properties: struct {
			Cpus *[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			} `json:"cpus,omitempty"`
			Id                       clModels.DeviceId                                                        `json:"id"`
			Interfaces               *[]clModels.DeviceCommunicationInterface                                 `json:"interfaces,omitempty"`
			Memory                   *string                                                                  `json:"memory,omitempty"`
			ModelNumber              string                                                                   `json:"modelNumber"`
			OtelCollector            *bool                                                                    `json:"otelCollector,omitempty"`
			Peripherals              *[]clModels.DevicePeripheral                                             `json:"peripherals,omitempty"`
			SerialNumber             string                                                                   `json:"serialNumber"`
			Storage                  *string                                                                  `json:"storage,omitempty"`
			SupportedDeploymentTypes *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes `json:"supportedDeploymentTypes,omitempty"`
			SupportedRuntimes        *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes        `json:"supportedRuntimes,omitempty"`
			Vendor                   string                                                                   `json:"vendor"`
		}{
			Id:           "test-device-001",
			Vendor:       vendor,
			ModelNumber:  "M-001",
			SerialNumber: "SN-001",
		},
	}
}

// ---------------------------------------------------------------------------
// Test fixture helpers
// ---------------------------------------------------------------------------

// baseDevice returns a minimal valid DeviceCapabilitiesManifest.
func baseDevice() *clModels.DeviceCapabilitiesManifest {
	return &clModels.DeviceCapabilitiesManifest{
		ApiVersion: "device.margo.org/v1alpha1",
		Kind:       clModels.DeviceCapabilitiesManifestKindDeviceCapabilitiesManifest,
		Properties: struct {
			Cpus *[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			} `json:"cpus,omitempty"`
			Id                       clModels.DeviceId                                                        `json:"id"`
			Interfaces               *[]clModels.DeviceCommunicationInterface                                 `json:"interfaces,omitempty"`
			Memory                   *string                                                                  `json:"memory,omitempty"`
			ModelNumber              string                                                                   `json:"modelNumber"`
			OtelCollector            *bool                                                                    `json:"otelCollector,omitempty"`
			Peripherals              *[]clModels.DevicePeripheral                                             `json:"peripherals,omitempty"`
			SerialNumber             string                                                                   `json:"serialNumber"`
			Storage                  *string                                                                  `json:"storage,omitempty"`
			SupportedDeploymentTypes *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes `json:"supportedDeploymentTypes,omitempty"`
			SupportedRuntimes        *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes        `json:"supportedRuntimes,omitempty"`
			Vendor                   string                                                                   `json:"vendor"`
		}{
			Id:           "test-device-001",
			Vendor:       "Acme Corp",
			ModelNumber:  "M-001",
			SerialNumber: "SN-001",
		},
	}
}

// engine returns a *propertySelectorEngine (concrete type) for white-box testing.
func engine(d *clModels.DeviceCapabilitiesManifest) *propertySelectorEngine {
	return New(d).(*propertySelectorEngine)
}

// ---------------------------------------------------------------------------
// Empty / root pointer
// ---------------------------------------------------------------------------

func TestResolvePointer_EmptyPointer_ReturnsWholeProperties(t *testing.T) {
	// RFC 6901: empty string refers to the whole document (properties struct here).
	d := baseDevice()
	e := engine(d)

	val, exists, err := e.resolvePointer("")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.NotNil(t, val)
}

// ---------------------------------------------------------------------------
// Top-level scalar fields
// ---------------------------------------------------------------------------

func TestResolvePointer_TopLevel_StringField_Found(t *testing.T) {
	d := baseDevice()
	d.Properties.Vendor = "EdgeCircuit Systems"
	e := engine(d)

	val, exists, err := e.resolvePointer("/vendor")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "EdgeCircuit Systems", val)
}

func TestResolvePointer_TopLevel_StringField_ExactValue(t *testing.T) {
	// Ensures the JSON round-trip does not alter string values.
	d := baseDevice()
	d.Properties.ModelNumber = "EF1.234.32"
	e := engine(d)

	val, exists, err := e.resolvePointer("/modelNumber")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "EF1.234.32", val)
}

func TestResolvePointer_TopLevel_BoolField_True(t *testing.T) {
	d := baseDevice()
	d.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(d)

	val, exists, err := e.resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, true, val)
}

func TestResolvePointer_TopLevel_BoolField_False(t *testing.T) {
	d := baseDevice()
	d.Properties.OtelCollector = pointers.Ptr(false)
	e := engine(d)

	val, exists, err := e.resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, false, val)
}

func TestResolvePointer_TopLevel_StringField_Memory(t *testing.T) {
	d := baseDevice()
	d.Properties.Memory = pointers.Ptr("64Gi")
	e := engine(d)

	val, exists, err := e.resolvePointer("/memory")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "64Gi", val)
}

// ---------------------------------------------------------------------------
// Absent / nil optional fields
// ---------------------------------------------------------------------------

func TestResolvePointer_AbsentOptionalField_NotFound(t *testing.T) {
	// OtelCollector is nil (omitempty) → key absent in marshaled JSON → not found.
	d := baseDevice()
	d.Properties.OtelCollector = nil
	e := engine(d)

	val, exists, err := e.resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestResolvePointer_NonExistentKey_NotFound(t *testing.T) {
	d := baseDevice()
	e := engine(d)

	val, exists, err := e.resolvePointer("/doesNotExist")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// Array fields (top-level)
// ---------------------------------------------------------------------------

func TestResolvePointer_ArrayField_ReturnsSlice(t *testing.T) {
	d := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	d.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/supportedDeploymentTypes")

	require.NoError(t, err)
	assert.True(t, exists)

	arr, ok := val.([]any)
	require.True(t, ok, "expected []any, got %T", val)
	assert.Len(t, arr, 2)
	assert.Equal(t, "compose", arr[0])
	assert.Equal(t, "custom", arr[1])
}

func TestResolvePointer_ArrayField_Peripherals_ReturnsObjectSlice(t *testing.T) {
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/peripherals")

	require.NoError(t, err)
	assert.True(t, exists)

	arr, ok := val.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 1)
}

// ---------------------------------------------------------------------------
// Nested pointer traversal (array index)
// ---------------------------------------------------------------------------

func TestResolvePointer_Nested_ArrayIndex_CoresAsFloat64(t *testing.T) {
	// After JSON round-trip all numbers become float64.
	d := baseDevice()
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: float32(4)},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/cpus/0/cores")

	require.NoError(t, err)
	assert.True(t, exists)
	// JSON unmarshal always produces float64 for numbers.
	assert.Equal(t, float64(4), val)
}

func TestResolvePointer_Nested_ArrayIndex_Architecture(t *testing.T) {
	d := baseDevice()
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: float32(8)},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/cpus/0/architecture")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "arm64", val)
}

func TestResolvePointer_Nested_SecondArrayElement(t *testing.T) {
	d := baseDevice()
	arch0 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arch1 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch0, Cores: 4},
		{Architecture: &arch1, Cores: 8},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/cpus/1/architecture")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "arm64", val)
}

func TestResolvePointer_Nested_OutOfBoundsIndex_NotFound(t *testing.T) {
	d := baseDevice()
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: 4},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/cpus/5/cores")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestResolvePointer_Nested_PeripheralObjectField(t *testing.T) {
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/peripherals/0/type")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "gpu", val)
}

func TestResolvePointer_Nested_PeripheralManufacturer(t *testing.T) {
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	val, exists, err := e.resolvePointer("/peripherals/0/manufacturer")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "NVIDIA", val)
}

// ---------------------------------------------------------------------------
// RFC 6901 token escaping
// ---------------------------------------------------------------------------

func TestResolvePointer_Tilde1_Unescaping(t *testing.T) {
	// ~1 in a pointer token represents '/'.
	// modelNumber contains a literal '/' — the pointer itself uses a plain token
	// since the key name "modelNumber" has no special chars; this test verifies
	// that a value containing '/' is returned verbatim (not confused with a separator).
	d := baseDevice()
	d.Properties.ModelNumber = "M/2000"
	e := engine(d)

	val, exists, err := e.resolvePointer("/modelNumber")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "M/2000", val)
}

func TestResolvePointer_Tilde0_Unescaping(t *testing.T) {
	// ~0 in a pointer token represents '~'.
	// serialNumber contains a literal '~' — returned verbatim.
	d := baseDevice()
	d.Properties.SerialNumber = "SN~001"
	e := engine(d)

	val, exists, err := e.resolvePointer("/serialNumber")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "SN~001", val)
}

func TestResolvePointer_DoubleSlash_TreatedAsEmptyToken(t *testing.T) {
	// "//" splits into tokens ["", ""] — the first empty token is a valid
	// RFC 6901 key (a field literally named ""). This should not panic and
	// should return not-found (no such field in Properties).
	d := baseDevice()
	e := engine(d)

	val, exists, err := e.resolvePointer("//")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// Idempotency — multiple calls produce consistent results
// ---------------------------------------------------------------------------

func TestResolvePointer_MultipleCallsConsistent(t *testing.T) {
	// Calling resolvePointer twice on the same engine must return the same result.
	// Guards against any accidental mutation of internal state between calls.
	d := baseDevice()
	d.Properties.Vendor = "EdgeCircuit Systems"
	e := engine(d)

	val1, exists1, err1 := e.resolvePointer("/vendor")
	val2, exists2, err2 := e.resolvePointer("/vendor")

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, exists1, exists2)
	assert.Equal(t, val1, val2)
}

func TestResolvePointer_DifferentPointers_IndependentResults(t *testing.T) {
	// Resolving two different pointers on the same engine must not interfere.
	d := baseDevice()
	d.Properties.Vendor = "Acme"
	d.Properties.ModelNumber = "X-100"
	e := engine(d)

	vendor, _, _ := e.resolvePointer("/vendor")
	model, _, _ := e.resolvePointer("/modelNumber")

	assert.Equal(t, "Acme", vendor)
	assert.Equal(t, "X-100", model)
}

// minimalDevice returns a DeviceCapabilitiesManifest with only the required fields set.
func minimalDevice() *clModels.DeviceCapabilitiesManifest {
	return &clModels.DeviceCapabilitiesManifest{
		ApiVersion: "device.margo.org/v1alpha1",
		Kind:       clModels.DeviceCapabilitiesManifestKindDeviceCapabilitiesManifest,
		Properties: struct {
			Cpus *[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			} `json:"cpus,omitempty"`
			Id                       clModels.DeviceId                                                        `json:"id"`
			Interfaces               *[]clModels.DeviceCommunicationInterface                                 `json:"interfaces,omitempty"`
			Memory                   *string                                                                  `json:"memory,omitempty"`
			ModelNumber              string                                                                   `json:"modelNumber"`
			OtelCollector            *bool                                                                    `json:"otelCollector,omitempty"`
			Peripherals              *[]clModels.DevicePeripheral                                             `json:"peripherals,omitempty"`
			SerialNumber             string                                                                   `json:"serialNumber"`
			Storage                  *string                                                                  `json:"storage,omitempty"`
			SupportedDeploymentTypes *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes `json:"supportedDeploymentTypes,omitempty"`
			SupportedRuntimes        *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes        `json:"supportedRuntimes,omitempty"`
			Vendor                   string                                                                   `json:"vendor"`
		}{
			Id:           "dev-001",
			Vendor:       "Acme Corp",
			ModelNumber:  "M-001",
			SerialNumber: "SN-001",
		},
	}
}

// ---------------------------------------------------------------------------
// Group 1: Single-key scalar object values  (/vendor, /modelNumber, …)
// ---------------------------------------------------------------------------

func TestResolvePointer_SingleKey_Vendor_Match(t *testing.T) {
	d := minimalDevice()
	d.Properties.Vendor = "EdgeCircuit Systems"

	val, exists, err := engine(d).resolvePointer("/vendor")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "EdgeCircuit Systems", val)
}

func TestResolvePointer_SingleKey_ModelNumber_Match(t *testing.T) {
	d := minimalDevice()
	d.Properties.ModelNumber = "EF1.234.32"

	val, exists, err := engine(d).resolvePointer("/modelNumber")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "EF1.234.32", val)
}

func TestResolvePointer_SingleKey_SerialNumber_Match(t *testing.T) {
	d := minimalDevice()
	d.Properties.SerialNumber = "SN12928342125"

	val, exists, err := engine(d).resolvePointer("/serialNumber")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "SN12928342125", val)
}

func TestResolvePointer_SingleKey_OtelCollector_True(t *testing.T) {
	d := minimalDevice()
	d.Properties.OtelCollector = pointers.Ptr(true)

	val, exists, err := engine(d).resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, true, val)
}

func TestResolvePointer_SingleKey_OtelCollector_False(t *testing.T) {
	d := minimalDevice()
	d.Properties.OtelCollector = pointers.Ptr(false)

	val, exists, err := engine(d).resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, false, val)
}

func TestResolvePointer_SingleKey_Memory_Match(t *testing.T) {
	d := minimalDevice()
	d.Properties.Memory = pointers.Ptr("64Gi")

	val, exists, err := engine(d).resolvePointer("/memory")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "64Gi", val)
}

func TestResolvePointer_SingleKey_Storage_Match(t *testing.T) {
	d := minimalDevice()
	d.Properties.Storage = pointers.Ptr("1862Gi")

	val, exists, err := engine(d).resolvePointer("/storage")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "1862Gi", val)
}

func TestResolvePointer_SingleKey_AbsentOptionalField_NotFound(t *testing.T) {
	// OtelCollector is nil → omitted from JSON → resolvePointer must return exists=false, no error.
	d := minimalDevice()
	d.Properties.OtelCollector = nil

	val, exists, err := engine(d).resolvePointer("/otelCollector")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestResolvePointer_SingleKey_NonExistentField_NotFound(t *testing.T) {
	d := minimalDevice()

	val, exists, err := engine(d).resolvePointer("/nonExistentField")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// Group 2: Array of objects — Peripherals
// Covers /peripherals (whole array), /peripherals/N (element), and
// /peripherals/N/<field> (field within element).
// ---------------------------------------------------------------------------

func peripheralDevice() *clModels.DeviceCapabilitiesManifest {
	d := minimalDevice()
	nvidiaManufacturer := "NVIDIA"
	nvidiaModel := "RTX 4090"
	logitechManufacturer := "Logitech"
	logitechModel := "C920"

	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{
			Type:         clModels.Gpu,
			Manufacturer: &nvidiaManufacturer,
			Model:        &nvidiaModel,
		},
		{
			Type:         clModels.Camera,
			Manufacturer: &logitechManufacturer,
			Model:        &logitechModel,
		},
	}
	return d
}

func TestResolvePointer_Peripherals_WholeArray_ReturnsSlice(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals")

	require.NoError(t, err)
	assert.True(t, exists)

	arr, ok := val.([]any)
	require.True(t, ok, "expected []any, got %T", val)
	assert.Len(t, arr, 2)
}

func TestResolvePointer_Peripherals_FirstElement_ReturnsObject(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/0")

	require.NoError(t, err)
	assert.True(t, exists)

	obj, ok := val.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", val)
	assert.Equal(t, "gpu", obj["type"])
}

func TestResolvePointer_Peripherals_FirstElement_Type(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/0/type")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "gpu", val)
}

func TestResolvePointer_Peripherals_FirstElement_Manufacturer(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/0/manufacturer")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "NVIDIA", val)
}

func TestResolvePointer_Peripherals_FirstElement_Model(t *testing.T) {
	// Core case: /peripherals/0/model — array-of-objects field traversal.
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/0/model")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "RTX 4090", val)
}

func TestResolvePointer_Peripherals_SecondElement_Type(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/1/type")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "camera", val)
}

func TestResolvePointer_Peripherals_SecondElement_Manufacturer(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/1/manufacturer")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "Logitech", val)
}

func TestResolvePointer_Peripherals_SecondElement_Model(t *testing.T) {
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/1/model")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "C920", val)
}

func TestResolvePointer_Peripherals_OutOfBoundsIndex_NotFound(t *testing.T) {
	// Array has 2 elements; index 5 must return exists=false, no error.
	d := peripheralDevice()

	val, exists, err := engine(d).resolvePointer("/peripherals/5/type")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestResolvePointer_Peripherals_AbsentOptionalField_Model_NotFound(t *testing.T) {
	// Peripheral with no Model set → "model" key absent in marshaled JSON.
	d := minimalDevice()
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu}, // Manufacturer and Model are nil → omitted
	}

	val, exists, err := engine(d).resolvePointer("/peripherals/0/model")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestResolvePointer_Peripherals_NilPeripherals_NotFound(t *testing.T) {
	// Peripherals field itself is nil → omitted from JSON → not found.
	d := minimalDevice()
	d.Properties.Peripherals = nil

	val, exists, err := engine(d).resolvePointer("/peripherals")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// Group 3: Array of objects — Cpus (nested numeric field after JSON round-trip)
// ---------------------------------------------------------------------------

func cpuDevice() *clModels.DeviceCapabilitiesManifest {
	d := minimalDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
		{Architecture: &arm64, Cores: float32(8)},
	}
	return d
}

func TestResolvePointer_Cpus_FirstElement_Architecture(t *testing.T) {
	d := cpuDevice()

	val, exists, err := engine(d).resolvePointer("/cpus/0/architecture")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "amd64", val)
}

func TestResolvePointer_Cpus_FirstElement_Cores_IsFloat64(t *testing.T) {
	// After JSON round-trip, float32(4) becomes float64(4).
	// This is the normalisation contract that HandleIn depends on.
	d := cpuDevice()

	val, exists, err := engine(d).resolvePointer("/cpus/0/cores")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.IsType(t, float64(0), val, "cores must be float64 after JSON round-trip")
	assert.Equal(t, float64(4), val)
}

func TestResolvePointer_Cpus_SecondElement_Architecture(t *testing.T) {
	d := cpuDevice()

	val, exists, err := engine(d).resolvePointer("/cpus/1/architecture")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "arm64", val)
}

func TestResolvePointer_Cpus_SecondElement_Cores_IsFloat64(t *testing.T) {
	d := cpuDevice()

	val, exists, err := engine(d).resolvePointer("/cpus/1/cores")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.IsType(t, float64(0), val)
	assert.Equal(t, float64(8), val)
}

// ---------------------------------------------------------------------------
// Group 4: Array of objects — Interfaces
// ---------------------------------------------------------------------------

func TestResolvePointer_Interfaces_FirstElement_Type(t *testing.T) {
	d := minimalDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}

	val, exists, err := engine(d).resolvePointer("/interfaces/0/type")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "ethernet", val)
}

func TestResolvePointer_Interfaces_SecondElement_Type(t *testing.T) {
	d := minimalDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}

	val, exists, err := engine(d).resolvePointer("/interfaces/1/type")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "wifi", val)
}

// ---------------------------------------------------------------------------
// Group 5: Invalid pointer format
// ---------------------------------------------------------------------------

func TestResolvePointer_MissingLeadingSlash_ReturnsError(t *testing.T) {
	d := minimalDevice()

	val, exists, err := engine(d).resolvePointer("vendor")

	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// Group 6: Idempotency across repeated calls
// ---------------------------------------------------------------------------

func TestResolvePointer_RepeatedCalls_SameResult(t *testing.T) {
	d := peripheralDevice()
	e := engine(d)

	val1, exists1, err1 := e.resolvePointer("/peripherals/0/model")
	val2, exists2, err2 := e.resolvePointer("/peripherals/0/model")

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, exists1, exists2)
	assert.Equal(t, val1, val2)
}

func TestResolvePointer_ScalarAndArrayOfObjects_IndependentResults(t *testing.T) {
	// Resolving a scalar key and an array-of-objects key on the same engine
	// must not interfere with each other.
	d := peripheralDevice()
	d.Properties.Vendor = "EdgeCircuit Systems"
	e := engine(d)

	vendor, _, _ := e.resolvePointer("/vendor")
	model, _, _ := e.resolvePointer("/peripherals/0/model")

	assert.Equal(t, "EdgeCircuit Systems", vendor)
	assert.Equal(t, "RTX 4090", model)
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

func TestHandleIn_ScalarString_Match(t *testing.T) {
	// /vendor resolves to "EdgeCircuit Systems"; values contains it → true
	device := deviceWithVendor("EdgeCircuit Systems")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_ScalarString_NoMatch(t *testing.T) {
	device := deviceWithVendor("Unknown Vendor")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ScalarString_CaseSensitive(t *testing.T) {
	// Spec: string comparisons MUST be exact and case-sensitive
	device := deviceWithVendor("edgecircuit systems")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("EdgeCircuit Systems"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ScalarNumber_Match(t *testing.T) {
	// /cpus/0/cores resolves to a float64 after JSON round-trip
	device := deviceWithVendor("Acme")
	cores := float32(4)
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: cores},
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.In,
		Values:   common.Vals(float64(4), float64(8)),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_ScalarNumber_NoMatch(t *testing.T) {
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: float32(2)},
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.In,
		Values:   common.Vals(float64(4), float64(8)),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ScalarBool_Match(t *testing.T) {
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = pointers.Ptr(true)
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.In,
		Values:   common.Vals(true),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_ScalarBool_NoMatch(t *testing.T) {
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = pointers.Ptr(false)
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.In,
		Values:   common.Vals(true),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ArrayOfStrings_ElementMatches(t *testing.T) {
	// /supportedDeploymentTypes is []string; at least one element must match a candidate
	device := deviceWithVendor("Acme")
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.In,
		Values:   common.Vals("custom"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_ArrayOfStrings_NoElementMatches(t *testing.T) {
	device := deviceWithVendor("Acme")
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.In,
		Values:   common.Vals("helm"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ArrayOfStrings_EmptyArray(t *testing.T) {
	// Empty array → nothing to match against → false
	device := deviceWithVendor("Acme")
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.In,
		Values:   common.Vals("compose"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_ArrayOfObjects_ReturnsFalse(t *testing.T) {
	// Per spec: array of objects MUST use ContainsAll/ContainsAny; In MUST return false
	device := deviceWithVendor("Acme")
	gpu := clModels.DevicePeripheralType("gpu")
	manufacturer := "NVIDIA"
	device.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: gpu, Manufacturer: &manufacturer},
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.In,
		Values:   common.Vals("gpu"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "ContainsAll or ContainsAny")
}

func TestHandleIn_KeyNotFound(t *testing.T) {
	// Pointer to a non-existent field → false with descriptive reason
	device := deviceWithVendor("Acme")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.In,
		Values:   common.Vals("someValue"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleIn_NilValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be present for In
	device := deviceWithVendor("Acme")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   nil,
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_EmptyValues_ReturnsFalse(t *testing.T) {
	// values present but empty slice → invalid per spec
	device := deviceWithVendor("Acme")
	engine := New(device).(*propertySelectorEngine)

	empty := []any{}
	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   &empty,
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_InvalidPointer_MissingLeadingSlash(t *testing.T) {
	// RFC 6901: pointer must start with '/' (unless empty string)
	device := deviceWithVendor("Acme")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "vendor", // missing leading slash
		Operator: clModels.In,
		Values:   common.Vals("Acme"),
	}

	ok, reason := engine.HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_NestedPointer_Match(t *testing.T) {
	// /cpus/0/architecture resolves through two levels of nesting
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: 4},
	}
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.In,
		Values:   common.Vals("arm64", "amd64"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_PointerEscaping_Tilde1(t *testing.T) {
	// RFC 6901: ~1 in a token represents '/'
	// This tests that the pointer "/modelNumber" works and that token unescaping
	// does not corrupt normal keys. A key containing '/' would need ~1 encoding.
	device := deviceWithVendor("Acme")
	device.Properties.ModelNumber = "M/2000"
	engine := New(device).(*propertySelectorEngine)

	// /modelNumber resolves to "M/2000" — no escaping needed in the pointer itself
	me := &clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.In,
		Values:   common.Vals("M/2000"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_PropertiesMapCachedAcrossCalls(t *testing.T) {
	// resolvePointer lazily initialises propertiesMap; calling HandleIn twice
	// must produce consistent results (cache is not corrupted between calls).
	device := deviceWithVendor("EdgeCircuit Systems")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("EdgeCircuit Systems"),
	}

	ok1, _ := engine.HandleIn(me)
	ok2, _ := engine.HandleIn(me)

	require.True(t, ok1)
	require.True(t, ok2)
}

func TestHandleIn_MultipleValuesFirstMatches(t *testing.T) {
	// Ensure iteration stops correctly when the first candidate matches
	device := deviceWithVendor("NanoEdge Devices")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("NanoEdge Devices", "EdgeCircuit Systems", "Acme"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_MultipleValuesLastMatches(t *testing.T) {
	device := deviceWithVendor("Acme")
	engine := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.In,
		Values:   common.Vals("NanoEdge Devices", "EdgeCircuit Systems", "Acme"),
	}

	ok, reason := engine.HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// HandleNotIn test suite
// ---------------------------------------------------------------------------

// --- Scalar string ---

func TestHandleNotIn_ScalarString_NotInValues_ReturnsTrue(t *testing.T) {
	// /vendor resolves to "UnknownVendor"; not in values list → true
	device := deviceWithVendor("UnknownVendor")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ScalarString_InValues_ReturnsFalse(t *testing.T) {
	// /vendor resolves to "EdgeCircuit Systems"; present in values → false
	device := deviceWithVendor("EdgeCircuit Systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_ScalarString_CaseSensitive_NotInValues_ReturnsTrue(t *testing.T) {
	// "edgecircuit systems" ≠ "EdgeCircuit Systems" → true (case-sensitive)
	device := deviceWithVendor("edgecircuit systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ScalarString_SingleValue_Match_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("Acme"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Scalar number ---

func TestHandleNotIn_ScalarNumber_NotInValues_ReturnsTrue(t *testing.T) {
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: float32(2)},
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.NotIn,
		Values:   common.Vals(float64(4), float64(8)),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ScalarNumber_InValues_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: float32(4)},
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.NotIn,
		Values:   common.Vals(float64(4), float64(8)),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Scalar bool ---

func TestHandleNotIn_ScalarBool_False_NotInValues_ReturnsTrue(t *testing.T) {
	// device has otelCollector=false; values=[true] → false not in [true] → true
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = pointers.Ptr(false)
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.NotIn,
		Values:   common.Vals(true),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ScalarBool_True_InValues_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.NotIn,
		Values:   common.Vals(true),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Array of scalars (strings) ---

func TestHandleNotIn_ArrayOfStrings_NoElementInValues_ReturnsTrue(t *testing.T) {
	// supportedDeploymentTypes=[compose, custom]; values=[helm] → none match → true
	device := deviceWithVendor("Acme")
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.NotIn,
		Values:   common.Vals("helm"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ArrayOfStrings_OneElementInValues_ReturnsFalse(t *testing.T) {
	// supportedDeploymentTypes=[compose, custom]; values=[custom] → "custom" matches → false
	device := deviceWithVendor("Acme")
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.NotIn,
		Values:   common.Vals("custom"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_ArrayOfStrings_EmptyArray_ReturnsFalse(t *testing.T) {
	// Empty array → nothing to match → false (spec: key must exist and no values match)
	device := deviceWithVendor("Acme")
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.NotIn,
		Values:   common.Vals("compose"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_ArrayOfStrings_MultipleValuesNoneMatch_ReturnsTrue(t *testing.T) {
	device := deviceWithVendor("Acme")
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.NotIn,
		Values:   common.Vals("helm", "custom"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Array of objects ---

func TestHandleNotIn_ArrayOfObjects_ReturnsFalse(t *testing.T) {
	// Per spec: array of objects MUST use ContainsAll/ContainsAny; NotIn MUST return false
	device := deviceWithVendor("Acme")
	manufacturer := "NVIDIA"
	device.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.NotIn,
		Values:   common.Vals("gpu"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "ContainsAll or ContainsAny")
}

// --- Key not found ---

func TestHandleNotIn_KeyNotFound_ReturnsFalse(t *testing.T) {
	// Spec: NotIn is true only when the key exists; absent key → false
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.NotIn,
		Values:   common.Vals("someValue"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleNotIn_AbsentOptionalField_ReturnsFalse(t *testing.T) {
	// OtelCollector is nil (omitempty) → absent from JSON → key not found → false
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = nil
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.NotIn,
		Values:   common.Vals(true),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Invalid values ---

func TestHandleNotIn_NilValues_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   nil,
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_EmptyValues_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	empty := []any{}
	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   &empty,
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_MixedTypeValues_ReturnsFalse(t *testing.T) {
	// values MUST be the same data type → mixed types are invalid
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("Acme", float64(42)),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_BoolMultipleValues_ReturnsFalse(t *testing.T) {
	// Boolean values MUST contain exactly one entry
	device := deviceWithVendor("Acme")
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.NotIn,
		Values:   common.Vals(true, false),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Invalid pointer ---

func TestHandleNotIn_InvalidPointer_MissingLeadingSlash_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "vendor", // missing leading '/'
		Operator: clModels.NotIn,
		Values:   common.Vals("Acme"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Nested pointer ---

func TestHandleNotIn_NestedPointer_NotInValues_ReturnsTrue(t *testing.T) {
	// /cpus/0/architecture = "amd64"; values=["arm64"] → not in → true
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: 4},
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.NotIn,
		Values:   common.Vals("arm64"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_NestedPointer_InValues_ReturnsFalse(t *testing.T) {
	// /cpus/0/architecture = "arm64"; values=["arm64", "amd64"] → in → false
	device := deviceWithVendor("Acme")
	arch := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &arch, Cores: 8},
	}
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.NotIn,
		Values:   common.Vals("arm64", "amd64"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_NestedPointer_OutOfBoundsIndex_ReturnsFalse(t *testing.T) {
	// /cpus/5/cores → key not found → false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/5/cores",
		Operator: clModels.NotIn,
		Values:   common.Vals(float64(4)),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Idempotency ---

func TestHandleNotIn_RepeatedCalls_ConsistentResults(t *testing.T) {
	device := deviceWithVendor("UnknownVendor")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems"),
	}

	ok1, reason1 := e.HandleNotIn(me)
	ok2, reason2 := e.HandleNotIn(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleNotIn_MultipleValuesFirstMatches_ReturnsFalse(t *testing.T) {
	// First candidate in values matches the device vendor → false immediately
	device := deviceWithVendor("EdgeCircuit Systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices", "Acme"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_MultipleValuesLastMatches_ReturnsFalse(t *testing.T) {
	// Last candidate in values matches → still false
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.NotIn,
		Values:   common.Vals("EdgeCircuit Systems", "NanoEdge Devices", "Acme"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Peripheral (array of objects, nested field) ---

func TestHandleNotIn_Peripheral_NestedField_NotInValues_ReturnsTrue(t *testing.T) {
	// /peripherals/0/manufacturer = "NVIDIA"; values=["Logitech"] → not in → true
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/manufacturer",
		Operator: clModels.NotIn,
		Values:   common.Vals("Logitech"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_Peripheral_NestedField_InValues_ReturnsFalse(t *testing.T) {
	// /peripherals/0/manufacturer = "NVIDIA"; values=["NVIDIA"] → in → false
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/manufacturer",
		Operator: clModels.NotIn,
		Values:   common.Vals("NVIDIA"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- modelNumber with special characters ---

func TestHandleNotIn_ModelNumber_WithSlash_NotInValues_ReturnsTrue(t *testing.T) {
	device := deviceWithVendor("Acme")
	device.Properties.ModelNumber = "M/2000"
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.NotIn,
		Values:   common.Vals("M/1000", "M/3000"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_ModelNumber_WithSlash_InValues_ReturnsFalse(t *testing.T) {
	device := deviceWithVendor("Acme")
	device.Properties.ModelNumber = "M/2000"
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.NotIn,
		Values:   common.Vals("M/2000"),
	}

	ok, reason := e.HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// HandleExists test suite
// ---------------------------------------------------------------------------

// --- Scalar string fields ---

func TestHandleExists_ScalarString_Vendor_Present_ReturnsTrue(t *testing.T) {
	// /vendor is always present (required field) → Exists must return true.
	device := deviceWithVendor("EdgeCircuit Systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_ScalarString_ModelNumber_Present_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.ModelNumber = "EF1.234.32"
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_ScalarString_SerialNumber_Present_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.SerialNumber = "SN12928342125"
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/serialNumber",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Optional scalar fields (present vs absent) ---

func TestHandleExists_OptionalBool_OtelCollector_Present_ReturnsTrue(t *testing.T) {
	// OtelCollector is set → present in JSON → Exists must return true.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_OptionalBool_OtelCollector_False_Present_ReturnsTrue(t *testing.T) {
	// OtelCollector=false is still present (not nil) → Exists must return true.
	// A zero value is not the same as absent.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(false)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_OptionalBool_OtelCollector_Nil_ReturnsFalse(t *testing.T) {
	// OtelCollector is nil (omitempty) → absent from JSON → Exists must return false.
	device := baseDevice()
	device.Properties.OtelCollector = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_OptionalString_Memory_Present_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.Memory = pointers.Ptr("64Gi")
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_OptionalString_Memory_Nil_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.Memory = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_OptionalString_Storage_Present_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.Storage = pointers.Ptr("1862Gi")
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_OptionalString_Storage_Nil_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.Storage = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Non-existent key ---

func TestHandleExists_NonExistentKey_ReturnsFalse(t *testing.T) {
	// A key that does not exist in properties at all → Exists must return false.
	device := baseDevice()
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Array fields ---

func TestHandleExists_ArrayField_SupportedDeploymentTypes_Present_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_ArrayField_SupportedDeploymentTypes_Nil_ReturnsFalse(t *testing.T) {
	// Nil pointer → omitted from JSON → key absent → false.
	device := baseDevice()
	device.Properties.SupportedDeploymentTypes = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_ArrayField_Peripherals_Present_ReturnsTrue(t *testing.T) {
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_ArrayField_Peripherals_Nil_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.Peripherals = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Nested pointer traversal ---

func TestHandleExists_Nested_CpuCores_Present_ReturnsTrue(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_Nested_CpuArchitecture_Present_ReturnsTrue(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_Nested_CpuArchitecture_Absent_ReturnsFalse(t *testing.T) {
	// CPU entry with no Architecture set → "architecture" key omitted from JSON.
	device := baseDevice()
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Cores: float32(4)}, // Architecture is nil → omitted
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_Nested_OutOfBoundsIndex_ReturnsFalse(t *testing.T) {
	// Array has 2 CPUs; index 5 does not exist → false.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/5/cores",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_Nested_PeripheralManufacturer_Present_ReturnsTrue(t *testing.T) {
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/manufacturer",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_Nested_PeripheralModel_Absent_ReturnsFalse(t *testing.T) {
	// Peripheral with no Model set → "model" key omitted from JSON → false.
	device := baseDevice()
	device.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu}, // Manufacturer and Model are nil → omitted
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/model",
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Spec: values MUST be omitted ---

func TestHandleExists_ValuesPresent_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be omitted for Exists; providing them is invalid.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Exists,
		Values:   common.Vals("Acme"), // must be rejected
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_EmptyValuesSlice_ReturnsFalse(t *testing.T) {
	// An explicitly empty (non-nil) values slice is still a violation of the spec.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	empty := []any{}
	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Exists,
		Values:   &empty,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleExists_NilValues_DoesNotViolateSpec(t *testing.T) {
	// nil Values is the correct form for Exists → must not be rejected.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Exists,
		Values:   nil,
	}

	ok, reason := e.HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Invalid pointer format ---

func TestHandleExists_InvalidPointer_MissingLeadingSlash_ReturnsError(t *testing.T) {
	// RFC 6901: pointer must start with '/' (unless empty string).
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "vendor", // missing leading '/'
		Operator: clModels.Exists,
	}

	ok, reason := e.HandleExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Idempotency ---

func TestHandleExists_RepeatedCalls_ConsistentResults(t *testing.T) {
	// Calling HandleExists twice on the same engine must produce identical results.
	// Guards against accidental mutation of internal state between calls.
	device := deviceWithVendor("EdgeCircuit Systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Exists,
	}

	ok1, reason1 := e.HandleExists(me)
	ok2, reason2 := e.HandleExists(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleExists_TwoIndependentKeys_DoNotInterfere(t *testing.T) {
	// Resolving two different keys on the same engine must not interfere.
	device := baseDevice()
	device.Properties.Memory = pointers.Ptr("64Gi")
	device.Properties.Storage = nil // absent
	e := engine(device)

	meMemory := &clModels.MatchExpression{Key: "/memory", Operator: clModels.Exists}
	meStorage := &clModels.MatchExpression{Key: "/storage", Operator: clModels.Exists}

	okMemory, _ := e.HandleExists(meMemory)
	okStorage, _ := e.HandleExists(meStorage)

	assert.True(t, okMemory)
	assert.False(t, okStorage)
}

// ---------------------------------------------------------------------------
// HandleDoesNotExists test suite
// ---------------------------------------------------------------------------

// --- Required scalar fields (always present → DoesNotExist must return false) ---

func TestHandleDoesNotExists_ScalarString_Vendor_Present_ReturnsFalse(t *testing.T) {
	// /vendor is a required field — always present in JSON → DoesNotExist must return false.
	device := deviceWithVendor("EdgeCircuit Systems")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_ScalarString_ModelNumber_Present_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.ModelNumber = "EF1.234.32"
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_ScalarString_SerialNumber_Present_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.SerialNumber = "SN12928342125"
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/serialNumber",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Optional scalar fields (present vs absent) ---

func TestHandleDoesNotExists_OptionalBool_OtelCollector_Present_ReturnsFalse(t *testing.T) {
	// OtelCollector is set → present in JSON → DoesNotExist must return false.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_OptionalBool_OtelCollector_False_Present_ReturnsFalse(t *testing.T) {
	// OtelCollector=false is still present (not nil) → DoesNotExist must return false.
	// A zero/false value is not the same as absent.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(false)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_OptionalBool_OtelCollector_Nil_ReturnsTrue(t *testing.T) {
	// OtelCollector is nil (omitempty) → absent from JSON → DoesNotExist must return true.
	device := baseDevice()
	device.Properties.OtelCollector = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_OptionalString_Memory_Present_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.Memory = pointers.Ptr("64Gi")
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_OptionalString_Memory_Nil_ReturnsTrue(t *testing.T) {
	// Memory is nil → omitted from JSON → key absent → DoesNotExist returns true.
	device := baseDevice()
	device.Properties.Memory = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_OptionalString_Storage_Present_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	device.Properties.Storage = pointers.Ptr("1862Gi")
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_OptionalString_Storage_Nil_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.Storage = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Completely non-existent key ---

func TestHandleDoesNotExists_NonExistentKey_ReturnsTrue(t *testing.T) {
	// A key that has never existed in properties → DoesNotExist must return true.
	device := baseDevice()
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Array fields (present vs nil) ---

func TestHandleDoesNotExists_ArrayField_SupportedDeploymentTypes_Present_ReturnsFalse(
	t *testing.T,
) {
	device := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_ArrayField_SupportedDeploymentTypes_Nil_ReturnsTrue(t *testing.T) {
	// Nil pointer → omitted from JSON → key absent → DoesNotExist returns true.
	device := baseDevice()
	device.Properties.SupportedDeploymentTypes = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_ArrayField_Peripherals_Present_ReturnsFalse(t *testing.T) {
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_ArrayField_Peripherals_Nil_ReturnsTrue(t *testing.T) {
	device := baseDevice()
	device.Properties.Peripherals = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Nested pointer traversal ---

func TestHandleDoesNotExists_Nested_CpuCores_Present_ReturnsFalse(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_Nested_CpuArchitecture_Present_ReturnsFalse(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_Nested_CpuArchitecture_Absent_ReturnsTrue(t *testing.T) {
	// CPU entry with no Architecture set → "architecture" key omitted from JSON → true.
	device := baseDevice()
	device.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Cores: float32(4)}, // Architecture is nil → omitted
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/architecture",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_Nested_OutOfBoundsIndex_ReturnsTrue(t *testing.T) {
	// Array has 2 CPUs; index 5 does not exist → key absent → DoesNotExist returns true.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/5/cores",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_Nested_PeripheralManufacturer_Present_ReturnsFalse(t *testing.T) {
	d := peripheralDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/manufacturer",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_Nested_PeripheralModel_Absent_ReturnsTrue(t *testing.T) {
	// Peripheral with no Model set → "model" key omitted from JSON → DoesNotExist returns true.
	device := baseDevice()
	device.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu}, // Manufacturer and Model are nil → omitted
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/peripherals/0/model",
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Spec: values MUST be omitted ---

func TestHandleDoesNotExists_ValuesPresent_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be omitted for DoesNotExist; providing them is invalid.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.DoesNotExist,
		Values:   common.Vals("Acme"), // must be rejected regardless of key presence
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_EmptyValuesSlice_ReturnsFalse(t *testing.T) {
	// An explicitly empty (non-nil) values slice is still a spec violation.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	empty := []any{}
	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.DoesNotExist,
		Values:   &empty,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleDoesNotExists_NilValues_DoesNotViolateSpec(t *testing.T) {
	// nil Values is the correct form for DoesNotExist → must not be rejected.
	// Key is absent so the overall result is true.
	device := baseDevice()
	device.Properties.Storage = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.DoesNotExist,
		Values:   nil,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_ValuesPresent_KeyAbsent_StillReturnsFalse(t *testing.T) {
	// Even when the key is absent, providing values is a spec violation that
	// must be caught before the key lookup — values guard runs first.
	device := baseDevice()
	device.Properties.Storage = nil // key would be absent
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.DoesNotExist,
		Values:   common.Vals("someValue"),
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Invalid pointer format ---

func TestHandleDoesNotExists_InvalidPointer_MissingLeadingSlash_ReturnsError(t *testing.T) {
	// RFC 6901: pointer must start with '/' (unless empty string).
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "vendor", // missing leading '/'
		Operator: clModels.DoesNotExist,
	}

	ok, reason := e.HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Idempotency ---

func TestHandleDoesNotExists_RepeatedCalls_ConsistentResults(t *testing.T) {
	// Calling HandleDoesNotExists twice on the same engine must produce identical results.
	// Guards against accidental mutation of internal state between calls.
	device := baseDevice()
	device.Properties.Storage = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/storage",
		Operator: clModels.DoesNotExist,
	}

	ok1, reason1 := e.HandleDoesNotExists(me)
	ok2, reason2 := e.HandleDoesNotExists(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleDoesNotExists_TwoIndependentKeys_DoNotInterfere(t *testing.T) {
	// Resolving two different keys on the same engine must not interfere.
	// /memory is present, /storage is absent.
	device := baseDevice()
	device.Properties.Memory = pointers.Ptr("64Gi")
	device.Properties.Storage = nil
	e := engine(device)

	meMemory := &clModels.MatchExpression{Key: "/memory", Operator: clModels.DoesNotExist}
	meStorage := &clModels.MatchExpression{Key: "/storage", Operator: clModels.DoesNotExist}

	okMemory, _ := e.HandleDoesNotExists(meMemory)
	okStorage, _ := e.HandleDoesNotExists(meStorage)

	// /memory is present → DoesNotExist = false
	assert.False(t, okMemory)
	// /storage is absent → DoesNotExist = true
	assert.True(t, okStorage)
}

// ---------------------------------------------------------------------------
// HandleGt test suite
// ---------------------------------------------------------------------------

// --- Scalar numeric field: /cpus/0/cores ---

func TestHandleGt_ScalarNumber_ValueGreater_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 8; threshold = 4 → 8 > 4 → true
	d := cpuDevice() // cpuDevice has cores: [4, 8]
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/1/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(4)),
	}

	ok, reason := e.HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_ScalarNumber_ValueEqual_ReturnsFalse(t *testing.T) {
	// Gt is strictly greater than; equal values must return false.
	// /cpus/0/cores = 4; threshold = 4 → 4 > 4 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(4)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_ScalarNumber_ValueLess_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 8 → 4 > 8 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(8)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_ScalarNumber_Float32Threshold_ReturnsTrue(t *testing.T) {
	// Threshold supplied as float32 (from generated model) must be coerced to float64.
	// /cpus/0/cores = 4; threshold = float32(2) → 4 > 2 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float32(2)),
	}

	ok, reason := e.HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_ScalarNumber_FractionalThreshold_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 3.5 → 4 > 3.5 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(3.5)),
	}

	ok, reason := e.HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_ScalarNumber_FractionalThreshold_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 4.5 → 4 > 4.5 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(4.5)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_ScalarNumber_ZeroThreshold_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 0 → 4 > 0 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(0)),
	}

	ok, reason := e.HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Non-numeric resolved type → must return false ---

func TestHandleGt_ResolvedString_ReturnsFalse(t *testing.T) {
	// /vendor resolves to a string → Gt requires a number → false.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_ResolvedBool_ReturnsFalse(t *testing.T) {
	// /otelCollector resolves to a bool → Gt requires a number → false.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_ResolvedArray_ReturnsFalse(t *testing.T) {
	// /supportedDeploymentTypes resolves to an array → Gt requires a number → false.
	device := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Key not found ---

func TestHandleGt_KeyNotFound_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleGt_OptionalField_Nil_ReturnsFalse(t *testing.T) {
	// Memory is nil → omitted from JSON → key absent → false.
	device := baseDevice()
	device.Properties.Memory = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_Nested_OutOfBoundsIndex_ReturnsFalse(t *testing.T) {
	// /cpus/5/cores → index out of bounds → key absent → false.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/5/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Spec: values validation ---

func TestPropertySelectorHandleGt_NilValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be present for Gt.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   nil,
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestPropertySelectorHandleGt_EmptyValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST contain exactly one number for Gt.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	empty := []interface{}{}
	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   &empty,
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_MultipleValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST only contain a single number for Gt.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(2), float64(4)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_NonNumericThreshold_String_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be parsable as a number; a string threshold is invalid.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals("4"),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleGt_NonNumericThreshold_Bool_ReturnsFalse(t *testing.T) {
	// A boolean threshold is not parsable as a number → invalid.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(true),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Invalid pointer format ---

func TestHandleGt_InvalidPointer_MissingLeadingSlash_ReturnsFalse(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "cpus/0/cores", // missing leading '/'
		Operator: clModels.Gt,
		Values:   common.Vals(float64(1)),
	}

	ok, reason := e.HandleGt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Idempotency ---

func TestHandleGt_RepeatedCalls_ConsistentResults(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(2)),
	}

	ok1, reason1 := e.HandleGt(me)
	ok2, reason2 := e.HandleGt(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleGt_TwoIndependentKeys_DoNotInterfere(t *testing.T) {
	// Evaluating Gt on two different numeric keys must not interfere.
	// /cpus/0/cores = 4 > 2 → true; /cpus/1/cores = 8 > 10 → false
	d := cpuDevice()
	e := engine(d)

	me0 := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(2)),
	}
	me1 := &clModels.MatchExpression{
		Key:      "/cpus/1/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(10)),
	}

	ok0, _ := e.HandleGt(me0)
	ok1, _ := e.HandleGt(me1)

	assert.True(t, ok0)
	assert.False(t, ok1)
}

// ---------------------------------------------------------------------------
// HandleLt test suite
// ---------------------------------------------------------------------------

// --- Scalar numeric field: /cpus/0/cores ---

func TestHandleLt_ScalarNumber_ValueLess_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 8 → 4 < 8 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(8)),
	}

	ok, reason := e.HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_ScalarNumber_ValueEqual_ReturnsFalse(t *testing.T) {
	// Lt is strictly less than; equal values must return false.
	// /cpus/0/cores = 4; threshold = 4 → 4 < 4 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(4)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_ScalarNumber_ValueGreater_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 2 → 4 < 2 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(2)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_ScalarNumber_Float32Threshold_ReturnsTrue(t *testing.T) {
	// Threshold supplied as float32 must be coerced to float64.
	// /cpus/0/cores = 4; threshold = float32(8) → 4 < 8 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float32(8)),
	}

	ok, reason := e.HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_ScalarNumber_FractionalThreshold_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 4.5 → 4 < 4.5 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(4.5)),
	}

	ok, reason := e.HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_ScalarNumber_FractionalThreshold_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 3.5 → 4 < 3.5 is false
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(3.5)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_ScalarNumber_LargeThreshold_ReturnsTrue(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 1000 → 4 < 1000 → true
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(1000)),
	}

	ok, reason := e.HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// --- Non-numeric resolved type → must return false ---

func TestHandleLt_ResolvedString_ReturnsFalse(t *testing.T) {
	// /vendor resolves to a string → Lt requires a number → false.
	device := deviceWithVendor("Acme")
	e := New(device).(*propertySelectorEngine)

	me := &clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(10)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_ResolvedBool_ReturnsFalse(t *testing.T) {
	// /otelCollector resolves to a bool → Lt requires a number → false.
	device := baseDevice()
	device.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(10)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_ResolvedArray_ReturnsFalse(t *testing.T) {
	// /supportedDeploymentTypes resolves to an array → Lt requires a number → false.
	device := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	device.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
	}
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(10)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Key not found ---

func TestHandleLt_KeyNotFound_ReturnsFalse(t *testing.T) {
	device := baseDevice()
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(10)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleLt_OptionalField_Nil_ReturnsFalse(t *testing.T) {
	// Memory is nil → omitted from JSON → key absent → false.
	device := baseDevice()
	device.Properties.Memory = nil
	e := engine(device)

	me := &clModels.MatchExpression{
		Key:      "/memory",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(10)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_Nested_OutOfBoundsIndex_ReturnsFalse(t *testing.T) {
	// /cpus/5/cores → index out of bounds → key absent → false.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/5/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(100)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Spec: values validation ---

func TestPropertySelectorHandleLt_NilValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be present for Lt.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   nil,
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestPropertySelectorHandleLt_EmptyValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST contain exactly one number for Lt.
	d := cpuDevice()
	e := engine(d)

	empty := []interface{}{}
	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   &empty,
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_MultipleValues_ReturnsFalse(t *testing.T) {
	// Spec: values MUST only contain a single number for Lt.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(8), float64(16)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_NonNumericThreshold_String_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be parsable as a number; a string threshold is invalid.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals("8"),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleLt_NonNumericThreshold_Bool_ReturnsFalse(t *testing.T) {
	// A boolean threshold is not parsable as a number → invalid.
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(false),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Invalid pointer format ---

func TestHandleLt_InvalidPointer_MissingLeadingSlash_ReturnsFalse(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "cpus/0/cores", // missing leading '/'
		Operator: clModels.Lt,
		Values:   common.Vals(float64(100)),
	}

	ok, reason := e.HandleLt(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// --- Idempotency ---

func TestHandleLt_RepeatedCalls_ConsistentResults(t *testing.T) {
	d := cpuDevice()
	e := engine(d)

	me := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(8)),
	}

	ok1, reason1 := e.HandleLt(me)
	ok2, reason2 := e.HandleLt(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleLt_TwoIndependentKeys_DoNotInterfere(t *testing.T) {
	// /cpus/0/cores = 4 < 6 → true; /cpus/1/cores = 8 < 6 → false
	d := cpuDevice()
	e := engine(d)

	me0 := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(6)),
	}
	me1 := &clModels.MatchExpression{
		Key:      "/cpus/1/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(6)),
	}

	ok0, _ := e.HandleLt(me0)
	ok1, _ := e.HandleLt(me1)

	assert.True(t, ok0)
	assert.False(t, ok1)
}

// ---------------------------------------------------------------------------
// Gt vs Lt symmetry
// ---------------------------------------------------------------------------

func TestGtAndLt_Symmetry_SameKeyAndThreshold(t *testing.T) {
	// For the same key and threshold, Gt and Lt must never both be true.
	// /cpus/0/cores = 4; threshold = 4 → both equal → both false.
	d := cpuDevice()
	e := engine(d)

	meGt := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(4)),
	}
	meLt := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(4)),
	}

	okGt, _ := e.HandleGt(meGt)
	okLt, _ := e.HandleLt(meLt)

	assert.False(t, okGt)
	assert.False(t, okLt)
}

func TestGtAndLt_Symmetry_ValueAboveThreshold(t *testing.T) {
	// /cpus/1/cores = 8; threshold = 4 → Gt=true, Lt=false.
	d := cpuDevice()
	e := engine(d)

	meGt := &clModels.MatchExpression{
		Key:      "/cpus/1/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(4)),
	}
	meLt := &clModels.MatchExpression{
		Key:      "/cpus/1/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(4)),
	}

	okGt, _ := e.HandleGt(meGt)
	okLt, _ := e.HandleLt(meLt)

	assert.True(t, okGt)
	assert.False(t, okLt)
}

func TestGtAndLt_Symmetry_ValueBelowThreshold(t *testing.T) {
	// /cpus/0/cores = 4; threshold = 8 → Gt=false, Lt=true.
	d := cpuDevice()
	e := engine(d)

	meGt := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Gt,
		Values:   common.Vals(float64(8)),
	}
	meLt := &clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.Lt,
		Values:   common.Vals(float64(8)),
	}

	okGt, _ := e.HandleGt(meGt)
	okLt, _ := e.HandleLt(meLt)

	assert.False(t, okGt)
	assert.True(t, okLt)
}

// ---------------------------------------------------------------------------
// HandleContainsAll test suite
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// ptr returns a pointer to the given value. Reused from existing test helpers.
// Already defined in properties_test.go — do not redeclare; shown here for reference only.
// func ptr[T any](v T) *T { return &v }

// containsAllPeripheralDevice builds a device with three peripherals to give
// richer AND-semantics coverage without repeating setup in every test.
//
//	index 0 → {type: gpu,      manufacturer: NVIDIA,    model: RTX 4090}
//	index 1 → {type: camera,   manufacturer: Logitech,  model: C920}
//	index 2 → {type: display,  manufacturer: Dell,      model: U2722D}
func containsAllPeripheralDevice() *clModels.DeviceCapabilitiesManifest {
	d := minimalDevice()
	nvidiaM := "NVIDIA"
	nvidiaModel := "RTX 4090"
	logitechM := "Logitech"
	logitechModel := "C920"
	dellM := "Dell"
	dellModel := "U2722D"

	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &nvidiaM, Model: &nvidiaModel},
		{Type: clModels.Camera, Manufacturer: &logitechM, Model: &logitechModel},
		{Type: clModels.Display, Manufacturer: &dellM, Model: &dellModel},
	}
	return d
}

// ---------------------------------------------------------------------------
// Group 1 — Spec: values MUST be omitted
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ValuesPresent_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be omitted for ContainsAll; providing them is invalid
	// regardless of whether itemSelector is valid.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		Values:   common.Vals("gpu"),
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_EmptyValuesSlice_ReturnsFalse(t *testing.T) {
	// An explicitly empty (non-nil) values slice is still a spec violation.
	// The guard must fire before any key resolution occurs.
	d := peripheralDevice()
	e := engine(d)

	empty := []any{}
	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		Values:   &empty,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_NilValues_DoesNotViolateSpec(t *testing.T) {
	// nil Values is the correct form for ContainsAll and must not be rejected.
	// The expression should proceed to key resolution and itemSelector evaluation.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		Values:   nil,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 2 — Spec: itemSelector MUST be present
// ---------------------------------------------------------------------------

func TestHandleContainsAll_NilItemSelector_ReturnsFalse(t *testing.T) {
	// Spec: itemSelector MUST be present for ContainsAll.
	// A nil itemSelector must be rejected before any key resolution.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:          "/peripherals",
		Operator:     clModels.ContainsAll,
		ItemSelector: nil,
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_EmptyItemSelectorExpressions_ReturnsFalse(t *testing.T) {
	// itemSelector is present but matchExpressions is empty → invalid per spec.
	// At least one expression is required.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ValuesPresent_NilItemSelector_ValuesGuardFiresFirst(t *testing.T) {
	// Both violations present: values non-nil AND itemSelector nil.
	// The values guard MUST fire first and reject before itemSelector is checked.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:          "/peripherals",
		Operator:     clModels.ContainsAll,
		Values:       common.Vals("gpu"),
		ItemSelector: nil,
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 3 — Key not found & nil optional fields
// ---------------------------------------------------------------------------

func TestHandleContainsAll_KeyNotFound_ReturnsFalse(t *testing.T) {
	// Pointer to a field that does not exist in properties at all →
	// resolvePointer returns exists=false → false with descriptive reason.
	d := baseDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleContainsAll_NilPeripherals_ReturnsFalse(t *testing.T) {
	// Peripherals field is nil (omitempty) → omitted from marshalled JSON →
	// resolvePointer returns exists=false → false.
	d := baseDevice()
	d.Properties.Peripherals = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_NilInterfaces_ReturnsFalse(t *testing.T) {
	// Interfaces field is nil → omitted from JSON → key absent → false.
	d := baseDevice()
	d.Properties.Interfaces = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_NilCpus_ReturnsFalse(t *testing.T) {
	// Cpus field is nil → omitted from JSON → key absent → false.
	d := baseDevice()
	d.Properties.Cpus = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 4 — Non-array resolved types
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ResolvedScalarString_ReturnsFalse(t *testing.T) {
	// /vendor resolves to a string scalar.
	// ContainsAll requires an array of objects → must return false.
	d := deviceWithVendor("Acme")
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ResolvedScalarBool_ReturnsFalse(t *testing.T) {
	// /otelCollector resolves to a bool scalar.
	// ContainsAll requires an array of objects → must return false.
	d := baseDevice()
	d.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ResolvedScalarNumber_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores resolves to float64 after JSON round-trip.
	// ContainsAll requires an array of objects → must return false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ResolvedScalarString_ModelNumber_ReturnsFalse(t *testing.T) {
	// /modelNumber resolves to a string scalar → false.
	d := baseDevice()
	d.Properties.ModelNumber = "EF1.234.32"
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 5 — Empty array
// ---------------------------------------------------------------------------

func TestHandleContainsAll_EmptyPeripheralsArray_ReturnsFalse(t *testing.T) {
	// Peripherals is an explicitly empty slice (not nil) →
	// nothing to iterate over → false.
	d := baseDevice()
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_EmptyInterfacesArray_ReturnsFalse(t *testing.T) {
	// Interfaces is an explicitly empty slice →
	// nothing to iterate over → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_EmptyCpusArray_ReturnsFalse(t *testing.T) {
	// Cpus is an explicitly empty slice →
	// nothing to iterate over → false.
	d := baseDevice()
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 6 — Array of scalars (must reject — use In/NotIn instead)
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ArrayOfScalars_SupportedDeploymentTypes_ReturnsFalse(t *testing.T) {
	// /supportedDeploymentTypes is []string (array of scalars).
	// ContainsAll requires an array of objects → must return false.
	d := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	d.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("compose")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ArrayOfScalars_SupportedRuntimes_ReturnsFalse(t *testing.T) {
	// /supportedRuntimes is []string (array of scalars).
	// ContainsAll requires an array of objects → must return false.
	d := baseDevice()
	oci := clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimesOci
	d.Properties.SupportedRuntimes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes{
		oci,
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/supportedRuntimes",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("oci")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 7 — Single-element array
// ---------------------------------------------------------------------------

func TestHandleContainsAll_SingleElementArray_ConditionMatches_ReturnsTrue(t *testing.T) {
	// Only one peripheral present; itemSelector matches it → true.
	d := baseDevice()
	manufacturer := "NVIDIA"
	model := "RTX 4090"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer, Model: &model},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleElementArray_ConditionNoMatch_ReturnsFalse(t *testing.T) {
	// Only one peripheral present (gpu); itemSelector looks for camera → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleElementArray_MultipleConditionsAllMatch_ReturnsTrue(t *testing.T) {
	// Only one peripheral; itemSelector has two conditions both satisfied → true.
	d := baseDevice()
	manufacturer := "NVIDIA"
	model := "RTX 4090"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer, Model: &model},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleElementArray_MultipleConditionsOneFails_ReturnsFalse(
	t *testing.T,
) {
	// Only one peripheral; second condition fails → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	model := "RTX 4090"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer, Model: &model},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("AMD")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 8 — Single condition: peripherals array
// ---------------------------------------------------------------------------

func TestHandleContainsAll_SingleCondition_FirstElementTypeMatches_ReturnsTrue(t *testing.T) {
	// peripherals=[{gpu,NVIDIA,RTX4090},{camera,Logitech,C920},{display,Dell,U2722D}]
	// /type In ["gpu"] → first element matches → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleCondition_SecondElementTypeMatches_ReturnsTrue(t *testing.T) {
	// /type In ["camera"] → second element matches → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ThirdElementTypeMatches_ReturnsTrue(t *testing.T) {
	// /type In ["display"] → third element matches → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("display")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleCondition_NoElementTypeMatches_ReturnsFalse(t *testing.T) {
	// /type In ["microphone"] → no element has type=microphone → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ManufacturerMatches_ReturnsTrue(t *testing.T) {
	// /manufacturer In ["Dell"] → third element matches → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ManufacturerNoMatch_ReturnsFalse(t *testing.T) {
	// /manufacturer In ["Samsung"] → no element has that manufacturer → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Samsung")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ModelMatches_ReturnsTrue(t *testing.T) {
	// /model In ["U2722D"] → third element (display/Dell) matches → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("U2722D")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ModelNoMatch_ReturnsFalse(t *testing.T) {
	// /model In ["GTX 1080"] → no element has that model → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("GTX 1080")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleCondition_TypeCaseSensitive_ReturnsFalse(t *testing.T) {
	// Spec: string comparisons MUST be exact and case-sensitive.
	// "GPU" != "gpu" → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("GPU")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleCondition_ManufacturerCaseSensitive_ReturnsFalse(t *testing.T) {
	// "nvidia" != "NVIDIA" → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("nvidia")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_SingleCondition_MultipleValuesInExpression_ReturnsTrue(t *testing.T) {
	// /type In ["microphone", "gpu", "speaker"] → first element type=gpu matches
	// one of the candidate values → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.In,
					Values:   common.Vals("microphone", "gpu", "speaker"),
				},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 9 — Multiple conditions: AND semantics
// ---------------------------------------------------------------------------

func TestHandleContainsAll_MultipleConditions_TwoConditions_FirstElementSatisfiesAll_ReturnsTrue(
	t *testing.T,
) {
	// /type In ["gpu"] AND /manufacturer In ["NVIDIA"]
	// First element {gpu,NVIDIA,RTX4090} satisfies both → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_TwoConditions_SecondElementSatisfiesAll_ReturnsTrue(
	t *testing.T,
) {
	// /type In ["camera"] AND /manufacturer In ["Logitech"]
	// Second element {camera,Logitech,C920} satisfies both → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_TwoConditions_ThirdElementSatisfiesAll_ReturnsTrue(
	t *testing.T,
) {
	// /type In ["display"] AND /manufacturer In ["Dell"]
	// Third element {display,Dell,U2722D} satisfies both → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("display")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_ConditionsSplitAcrossElements_ReturnsFalse(
	t *testing.T,
) {
	// /type In ["gpu"] AND /manufacturer In ["Logitech"]
	// gpu→NVIDIA, camera→Logitech: conditions are satisfied by DIFFERENT elements.
	// ContainsAll requires a SINGLE element to satisfy ALL conditions → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_TypeAndModelSplitAcrossElements_ReturnsFalse(
	t *testing.T,
) {
	// /type In ["gpu"] AND /model In ["C920"]
	// gpu→RTX4090, camera→C920: no single element satisfies both → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("C920")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_ThreeConditionsAllSatisfiedByFirstElement_ReturnsTrue(
	t *testing.T,
) {
	// /type In ["gpu"] AND /manufacturer In ["NVIDIA"] AND /model In ["RTX 4090"]
	// First element satisfies all three → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("RTX 4090")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_ThreeConditionsLastFails_ReturnsFalse(t *testing.T) {
	// /type In ["gpu"] AND /manufacturer In ["NVIDIA"] AND /model In ["GTX 1080"]
	// First element has model "RTX 4090" not "GTX 1080" → no element satisfies all → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("GTX 1080")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_ThreeConditionsAllSatisfiedByThirdElement_ReturnsTrue(
	t *testing.T,
) {
	// /type In ["display"] AND /manufacturer In ["Dell"] AND /model In ["U2722D"]
	// Third element satisfies all three → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("display")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("U2722D")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_NoElementSatisfiesAnyCondition_ReturnsFalse(
	t *testing.T,
) {
	// /type In ["speaker"] AND /manufacturer In ["Sony"]
	// No element has type=speaker or manufacturer=Sony → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("speaker")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_MultipleConditions_FirstConditionMatchesAllElements_SecondMatchesNone_ReturnsFalse(
	t *testing.T,
) {
	// /type In ["gpu","camera","display"] (matches all) AND /manufacturer In ["Sony"] (matches none)
	// No single element satisfies both → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.In,
					Values:   common.Vals("gpu", "camera", "display"),
				},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 10 — Operator variety inside itemSelector: Exists
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ItemSelector_Exists_FieldPresent_ReturnsTrue(t *testing.T) {
	// /manufacturer Exists → first element has manufacturer set → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_Exists_FieldAbsent_ReturnsFalse(t *testing.T) {
	// /model Exists → peripheral has no model set → model key absent → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	// Model intentionally omitted → nil → omitted from JSON.
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 11 — Operator variety inside itemSelector: DoesNotExist
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ItemSelector_DoesNotExist_FieldAbsent_ReturnsTrue(t *testing.T) {
	// /model DoesNotExist → peripheral has no model set → key absent → true.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_DoesNotExist_FieldPresent_ReturnsFalse(t *testing.T) {
	// /manufacturer DoesNotExist → all peripherals have manufacturer set →
	// no element satisfies DoesNotExist → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 12 — Operator variety inside itemSelector: NotIn
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ItemSelector_NotIn_ElementNotInValues_ReturnsTrue(t *testing.T) {
	// /type NotIn ["microphone","speaker"] →
	// first element type=gpu is not in that list → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.NotIn,
					Values:   common.Vals("microphone", "speaker"),
				},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_NotIn_AllElementsInValues_ReturnsFalse(t *testing.T) {
	// /type NotIn ["gpu","camera","display"] →
	// all three elements have types in that list →
	// no element satisfies NotIn → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.NotIn,
					Values:   common.Vals("gpu", "camera", "display"),
				},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 13 — Operator variety inside itemSelector: Gt
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ItemSelector_Gt_CoresGreaterThanThreshold_ReturnsTrue(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /cores Gt [6] → second element cores=8 > 6 → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_Gt_NoCoresGreaterThanThreshold_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /cores Gt [10] → neither element has cores > 10 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(10))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 14 — Operator variety inside itemSelector: Lt
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ItemSelector_Lt_CoresLessThanThreshold_ReturnsTrue(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /cores Lt [5] → first element cores=4 < 5 → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(5))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_Lt_NoCoresLessThanThreshold_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /cores Lt [3] → neither element has cores < 3 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(3))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ItemSelector_GtAndIn_CombinedConditions_ReturnsTrue(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /architecture In ["arm64"] AND /cores Gt [6]
	// Second element: arm64 ✓, cores=8 > 6 ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ItemSelector_LtAndIn_CombinedConditions_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /architecture In ["amd64"] AND /cores Lt [3]
	// amd64 element has cores=4, not < 3 → no single element satisfies both → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(3))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 15 — Cpus array: single condition
// ---------------------------------------------------------------------------

func TestHandleContainsAll_CpusArray_ArchAmd64_Matches_ReturnsTrue(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// /architecture In ["amd64"] → first element matches → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_CpusArray_ArchArm64_Matches_ReturnsTrue(t *testing.T) {
	// /architecture In ["arm64"] → second element matches → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_CpusArray_ArchRiscv64_NoMatch_ReturnsFalse(t *testing.T) {
	// /architecture In ["riscv64"] → neither element matches → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_CpusArray_CoresExactMatch_ReturnsTrue(t *testing.T) {
	// /cores In [float64(4)] → first element cores=4 matches → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.In, Values: common.Vals(float64(4))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_CpusArray_CoresNoMatch_ReturnsFalse(t *testing.T) {
	// /cores In [float64(16)] → neither element has cores=16 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.In, Values: common.Vals(float64(16))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 16 — Cpus array: multiple conditions
// ---------------------------------------------------------------------------

func TestHandleContainsAll_CpusArray_ArchAndCores_BothSatisfiedBySecondElement_ReturnsTrue(
	t *testing.T,
) {
	// /architecture In ["arm64"] AND /cores Gt [6]
	// Second element: arm64 ✓, cores=8 > 6 ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_CpusArray_ArchAndCores_BothSatisfiedByFirstElement_ReturnsTrue(
	t *testing.T,
) {
	// /architecture In ["amd64"] AND /cores Lt [6]
	// First element: amd64 ✓, cores=4 < 6 ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_CpusArray_ArchAndCores_SplitAcrossElements_ReturnsFalse(t *testing.T) {
	// /architecture In ["amd64"] AND /cores Gt [6]
	// amd64 element has cores=4 (not > 6); arm64 element has cores=8 but arch≠amd64.
	// No single element satisfies both → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_CpusArray_ArchExistsAndCoresGt_ReturnsTrue(t *testing.T) {
	// /architecture Exists AND /cores Gt [3]
	// First element: architecture=amd64 ✓, cores=4 > 3 ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.Exists},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(3))},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 17 — Interfaces array
// ---------------------------------------------------------------------------

func TestHandleContainsAll_InterfacesArray_EthernetMatches_ReturnsTrue(t *testing.T) {
	// /interfaces=[{ethernet},{wifi}]; /type In ["ethernet"] → first element matches → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_InterfacesArray_WifiMatches_ReturnsTrue(t *testing.T) {
	// /type In ["wifi"] → second element matches → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("wifi")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_InterfacesArray_BluetoothNoMatch_ReturnsFalse(t *testing.T) {
	// /type In ["bluetooth"] → neither element has type=bluetooth → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("bluetooth")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_InterfacesArray_MultipleTypes_OneMatches_ReturnsTrue(t *testing.T) {
	// /interfaces=[{ethernet},{wifi},{cellular}]
	// /type In ["cellular"] → third element matches → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
		{Type: clModels.Cellular},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("cellular")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_InterfacesArray_TypeNotInList_ReturnsFalse(t *testing.T) {
	// /interfaces=[{ethernet},{wifi}]
	// /type NotIn ["ethernet","wifi"] → both elements have types in the list →
	// no element satisfies NotIn → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.NotIn, Values: common.Vals("ethernet", "wifi")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 18 — Invalid pointer format
// ---------------------------------------------------------------------------

func TestHandleContainsAll_InvalidPointer_MissingLeadingSlash_ReturnsFalse(t *testing.T) {
	// RFC 6901: pointer must start with '/'.
	// "peripherals" without leading slash → resolvePointer returns error → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "peripherals", // missing leading '/'
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_InvalidPointer_MissingLeadingSlash_Cpus_ReturnsFalse(t *testing.T) {
	// "cpus" without leading slash → resolvePointer returns error → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "cpus", // missing leading '/'
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_InvalidPointer_MissingLeadingSlash_Interfaces_ReturnsFalse(
	t *testing.T,
) {
	// "interfaces" without leading slash → resolvePointer returns error → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "interfaces", // missing leading '/'
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 19 — Case sensitivity in itemSelector key values
// ---------------------------------------------------------------------------

func TestHandleContainsAll_CaseSensitivity_TypeUpperCase_ReturnsFalse(t *testing.T) {
	// Spec: string comparisons MUST be exact and case-sensitive.
	// "GPU" != "gpu" → no element matches → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("GPU")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_CaseSensitivity_ManufacturerLowerCase_ReturnsFalse(t *testing.T) {
	// "nvidia" != "NVIDIA" → no element matches → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("nvidia")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_CaseSensitivity_ModelMixedCase_ReturnsFalse(t *testing.T) {
	// "rtx 4090" != "RTX 4090" → no element matches → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("rtx 4090")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_CaseSensitivity_ArchitectureLowerCase_Matches_ReturnsTrue(t *testing.T) {
	// "amd64" is already lowercase and matches the stored value exactly → true.
	// Guards against over-normalisation in the implementation.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 20 — Absent optional field inside an array element
// ---------------------------------------------------------------------------

func TestHandleContainsAll_AbsentOptionalField_ModelNil_InOperator_ReturnsFalse(t *testing.T) {
	// Peripheral has no Model set → "model" key absent in JSON for that element.
	// itemSelector: /model In ["RTX 4090"] → key absent → HandleIn returns false
	// for that element → no element satisfies → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	// Model intentionally omitted.
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("RTX 4090")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_AbsentOptionalField_ModelNil_DoesNotExist_ReturnsTrue(t *testing.T) {
	// Peripheral has no Model set → "model" key absent.
	// itemSelector: /model DoesNotExist → key absent → true for that element → true.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_AbsentOptionalField_ArchitectureNil_DoesNotExist_ReturnsTrue(
	t *testing.T,
) {
	// CPU entry with no Architecture set → "architecture" key absent in JSON.
	// itemSelector: /architecture DoesNotExist → key absent → true for that element → true.
	d := baseDevice()
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Cores: float32(4)}, // Architecture is nil → omitted from JSON
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_AbsentOptionalField_ArchitectureNil_ExistsOperator_ReturnsFalse(
	t *testing.T,
) {
	// CPU entry with no Architecture set → "architecture" key absent.
	// itemSelector: /architecture Exists → key absent → false for that element.
	// Only one element in array → no element satisfies → false.
	d := baseDevice()
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Cores: float32(4)}, // Architecture is nil → omitted from JSON
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_MixedElements_SomeWithModelSomeWithout_ModelExists_ReturnsTrue(
	t *testing.T,
) {
	// Two peripherals: first has no model, second has model set.
	// itemSelector: /model Exists → second element satisfies → true.
	d := baseDevice()
	manufacturer1 := "NVIDIA"
	manufacturer2 := "Logitech"
	model2 := "C920"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer1},                    // no model
		{Type: clModels.Camera, Manufacturer: &manufacturer2, Model: &model2}, // has model
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 21 — Idempotency: repeated calls on the same engine
// ---------------------------------------------------------------------------

func TestHandleContainsAll_RepeatedCalls_SameResult_ReturnsTrue(t *testing.T) {
	// Calling HandleContainsAll twice on the same engine with a matching
	// expression must produce identical true results.
	// Guards against accidental mutation of internal state between calls.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me)
	ok2, reason2 := e.HandleContainsAll(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleContainsAll_RepeatedCalls_SameResult_ReturnsFalse(t *testing.T) {
	// Calling HandleContainsAll twice on the same engine with a non-matching
	// expression must produce identical false results.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me)
	ok2, reason2 := e.HandleContainsAll(me)

	require.False(t, ok1)
	require.False(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleContainsAll_RepeatedCalls_CpusArray_SameResult_ReturnsTrue(t *testing.T) {
	// Repeated calls on cpus array must produce consistent results.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me)
	ok2, reason2 := e.HandleContainsAll(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

// ---------------------------------------------------------------------------
// Group 22 — Two independent ContainsAll expressions do not interfere
// ---------------------------------------------------------------------------

func TestHandleContainsAll_TwoIndependentExpressions_BothTrue_DoNotInterfere(t *testing.T) {
	// Evaluating two different ContainsAll expressions on the same engine
	// must not corrupt state between calls.
	// expr1: /peripherals → /type In ["gpu"]       → true
	// expr2: /peripherals → /manufacturer In ["Dell"] → true (display element)
	d := containsAllPeripheralDevice()
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me1)
	ok2, reason2 := e.HandleContainsAll(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.True(t, ok2)
	assert.Empty(t, reason2)
}

func TestHandleContainsAll_TwoIndependentExpressions_FirstTrueSecondFalse_DoNotInterfere(
	t *testing.T,
) {
	// expr1: /peripherals → /type In ["gpu"]          → true
	// expr2: /peripherals → /manufacturer In ["Sony"] → false
	// Results must be independent.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me1)
	ok2, reason2 := e.HandleContainsAll(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.False(t, ok2)
	assert.NotEmpty(t, reason2)
}

func TestHandleContainsAll_TwoIndependentExpressions_DifferentArrays_DoNotInterfere(t *testing.T) {
	// expr1: /peripherals → /type In ["gpu"]       → true
	// expr2: /cpus        → /architecture In ["amd64"] → true
	// Different arrays on the same engine must not interfere.
	d := containsAllPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me1)
	ok2, reason2 := e.HandleContainsAll(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.True(t, ok2)
	assert.Empty(t, reason2)
}

func TestHandleContainsAll_TwoIndependentExpressions_DifferentArrays_FirstTrueSecondFalse(
	t *testing.T,
) {
	// expr1: /peripherals → /type In ["gpu"]              → true
	// expr2: /cpus        → /architecture In ["riscv64"]  → false
	d := containsAllPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAll(me1)
	ok2, reason2 := e.HandleContainsAll(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.False(t, ok2)
	assert.NotEmpty(t, reason2)
}

// ---------------------------------------------------------------------------
// Group 23 — Evaluate integration: ContainsAll inside a full Selector
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ViaEvaluate_SingleContainsAllExpression_ReturnsTrue(t *testing.T) {
	// Evaluate a Selector containing a single ContainsAll expression.
	// /peripherals ContainsAll {/type In ["gpu"]} → true.
	d := containsAllPeripheralDevice()
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndInExpression_BothTrue_ReturnsTrue(
	t *testing.T,
) {
	// Selector with two expressions ANDed together:
	// 1. /vendor In ["Acme Corp"]                              → true
	// 2. /peripherals ContainsAll {/type In ["gpu"]}           → true
	// Both true → Evaluate returns true.
	d := containsAllPeripheralDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndInExpression_ContainsAllFails_ReturnsFalse(
	t *testing.T,
) {
	// Selector with two expressions ANDed together:
	// 1. /vendor In ["Acme Corp"]                                    → true
	// 2. /peripherals ContainsAll {/type In ["microphone"]}          → false
	// Second expression fails → Evaluate returns false.
	d := containsAllPeripheralDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndInExpression_InFails_ReturnsFalse(
	t *testing.T,
) {
	// Selector with two expressions ANDed together:
	// 1. /vendor In ["Unknown Vendor"]                         → false
	// 2. /peripherals ContainsAll {/type In ["gpu"]}           → true
	// First expression fails → Evaluate returns false immediately.
	d := containsAllPeripheralDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Unknown Vendor"),
			},
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 24 — Evaluate integration: ContainsAll combined with Gt/Lt/Exists
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndGt_BothTrue_ReturnsTrue(t *testing.T) {
	// Selector with two expressions ANDed:
	// 1. /cpus ContainsAll {/architecture In ["arm64"] AND /cores Gt [6]} → true
	// 2. /vendor In ["Acme Corp"]                                          → true
	// Both true → Evaluate returns true.
	d := cpuDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/cpus",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
						{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
					},
				},
			},
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndGt_ContainsAllFails_ReturnsFalse(
	t *testing.T,
) {
	// Selector with two expressions ANDed:
	// 1. /cpus ContainsAll {/architecture In ["riscv64"]} → false
	// 2. /vendor In ["Acme Corp"]                         → true
	// First fails → Evaluate returns false.
	d := cpuDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/cpus",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{
							Key:      "/architecture",
							Operator: clModels.In,
							Values:   common.Vals("riscv64"),
						},
					},
				},
			},
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndExists_BothTrue_ReturnsTrue(t *testing.T) {
	// Selector with two expressions ANDed:
	// 1. /peripherals ContainsAll {/manufacturer Exists} → true (all have manufacturer)
	// 2. /otelCollector Exists                           → true
	d := containsAllPeripheralDevice()
	d.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/manufacturer", Operator: clModels.Exists},
					},
				},
			},
			{
				Key:      "/otelCollector",
				Operator: clModels.Exists,
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_ContainsAllAndExists_ExistsFails_ReturnsFalse(t *testing.T) {
	// Selector with two expressions ANDed:
	// 1. /peripherals ContainsAll {/type In ["gpu"]} → true
	// 2. /otelCollector Exists                       → false (nil → absent)
	d := containsAllPeripheralDevice()
	d.Properties.OtelCollector = nil
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
			{
				Key:      "/otelCollector",
				Operator: clModels.Exists,
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 25 — Evaluate integration: multiple ContainsAll in one Selector
// ---------------------------------------------------------------------------

func TestHandleContainsAll_ViaEvaluate_TwoContainsAllExpressions_BothTrue_ReturnsTrue(
	t *testing.T,
) {
	// Selector with two ContainsAll expressions ANDed:
	// 1. /peripherals ContainsAll {/type In ["gpu"]}           → true
	// 2. /cpus        ContainsAll {/architecture In ["amd64"]} → true
	d := containsAllPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
			{
				Key:      "/cpus",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_ViaEvaluate_TwoContainsAllExpressions_SecondFails_ReturnsFalse(
	t *testing.T,
) {
	// Selector with two ContainsAll expressions ANDed:
	// 1. /peripherals ContainsAll {/type In ["gpu"]}              → true
	// 2. /cpus        ContainsAll {/architecture In ["riscv64"]}  → false
	// Second fails → Evaluate returns false.
	d := containsAllPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
			{
				Key:      "/cpus",
				Operator: clModels.ContainsAll,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{
							Key:      "/architecture",
							Operator: clModels.In,
							Values:   common.Vals("riscv64"),
						},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 26 — Edge cases
// ---------------------------------------------------------------------------

func TestHandleContainsAll_OutOfBoundsIndexInKey_ReturnsFalse(t *testing.T) {
	// /peripherals/5 → index 5 does not exist in a 3-element array →
	// resolvePointer returns exists=false → false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals/5",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_EmptyStringKey_ReturnsWholePropertiesNotArray_ReturnsFalse(
	t *testing.T,
) {
	// RFC 6901: empty string refers to the whole document (properties struct).
	// The whole properties struct is not an array of objects →
	// ContainsAll must return false.
	d := containsAllPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_LargePeripheralArray_MatchOnLastElement_ReturnsTrue(t *testing.T) {
	// Array with many peripherals; only the last one satisfies the condition.
	// Verifies the implementation iterates through all elements.
	d := baseDevice()
	m1 := "Vendor1"
	m2 := "Vendor2"
	m3 := "Vendor3"
	targetM := "TargetVendor"
	targetModel := "TargetModel"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Camera, Manufacturer: &m1},
		{Type: clModels.Display, Manufacturer: &m2},
		{Type: clModels.Microphone, Manufacturer: &m3},
		{Type: clModels.Gpu, Manufacturer: &targetM, Model: &targetModel},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("TargetVendor")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("TargetModel")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAll_LargePeripheralArray_NoElementMatchesAll_ReturnsFalse(t *testing.T) {
	// Array with many peripherals; none satisfies all conditions simultaneously.
	d := baseDevice()
	m1 := "Vendor1"
	m2 := "Vendor2"
	m3 := "Vendor3"
	m4 := "Vendor4"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Camera, Manufacturer: &m1},
		{Type: clModels.Display, Manufacturer: &m2},
		{Type: clModels.Microphone, Manufacturer: &m3},
		{Type: clModels.Speaker, Manufacturer: &m4},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAll_ItemSelectorKeyAbsentInAllElements_ReturnsFalse(t *testing.T) {
	// itemSelector references a key (/model) that is absent in ALL elements.
	// /model In ["RTX 4090"] → key absent in every element → no element satisfies → false.
	d := baseDevice()
	m1 := "NVIDIA"
	m2 := "Logitech"
	// No Model set on any peripheral.
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &m1},
		{Type: clModels.Camera, Manufacturer: &m2},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("RTX 4090")},
			},
		},
	}

	ok, reason := e.HandleContainsAll(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// HandleContainsAny test suite
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// containsAnyPeripheralDevice builds a device with three peripherals to give
// richer OR-semantics coverage without repeating setup in every test.
//
//	index 0 → {type: gpu,      manufacturer: NVIDIA,    model: RTX 4090}
//	index 1 → {type: camera,   manufacturer: Logitech,  model: C920}
//	index 2 → {type: display,  manufacturer: Dell,      model: U2722D}
//
// Reuses minimalDevice() from properties_test.go.
func containsAnyPeripheralDevice() *clModels.DeviceCapabilitiesManifest {
	d := minimalDevice()
	nvidiaM := "NVIDIA"
	nvidiaModel := "RTX 4090"
	logitechM := "Logitech"
	logitechModel := "C920"
	dellM := "Dell"
	dellModel := "U2722D"

	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &nvidiaM, Model: &nvidiaModel},
		{Type: clModels.Camera, Manufacturer: &logitechM, Model: &logitechModel},
		{Type: clModels.Display, Manufacturer: &dellM, Model: &dellModel},
	}
	return d
}

// ---------------------------------------------------------------------------
// Group 1 — Spec: values MUST be omitted
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ValuesPresent_ReturnsFalse(t *testing.T) {
	// Spec: values MUST be omitted for ContainsAny; providing them is invalid
	// regardless of whether itemSelector is valid.
	// The values guard must fire before any key resolution occurs.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		Values:   common.Vals("gpu"),
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_EmptyValuesSlice_ReturnsFalse(t *testing.T) {
	// An explicitly empty (non-nil) values slice is still a spec violation.
	// The guard must fire before any key resolution occurs.
	d := peripheralDevice()
	e := engine(d)

	empty := []any{}
	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		Values:   &empty,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_NilValues_DoesNotViolateSpec(t *testing.T) {
	// nil Values is the correct form for ContainsAny and must not be rejected.
	// The expression should proceed to key resolution and itemSelector evaluation.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		Values:   nil,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 2 — Spec: itemSelector MUST be present
// ---------------------------------------------------------------------------

func TestHandleContainsAny_NilItemSelector_ReturnsFalse(t *testing.T) {
	// Spec: itemSelector MUST be present for ContainsAny.
	// A nil itemSelector must be rejected before any key resolution.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:          "/peripherals",
		Operator:     clModels.ContainsAny,
		ItemSelector: nil,
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_EmptyItemSelectorExpressions_ReturnsFalse(t *testing.T) {
	// itemSelector is present but matchExpressions is empty → invalid per spec.
	// At least one expression is required.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ValuesPresent_NilItemSelector_ValuesGuardFiresFirst(t *testing.T) {
	// Both violations present: values non-nil AND itemSelector nil.
	// The values guard MUST fire first and reject before itemSelector is checked.
	d := peripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:          "/peripherals",
		Operator:     clModels.ContainsAny,
		Values:       common.Vals("gpu"),
		ItemSelector: nil,
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 3 — Key not found & nil optional fields
// ---------------------------------------------------------------------------

func TestHandleContainsAny_KeyNotFound_ReturnsFalse(t *testing.T) {
	// Pointer to a field that does not exist in properties at all →
	// resolvePointer returns exists=false → false with descriptive reason.
	d := baseDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/nonExistentField",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not found")
}

func TestHandleContainsAny_NilPeripherals_ReturnsFalse(t *testing.T) {
	// Peripherals field is nil (omitempty) → omitted from marshalled JSON →
	// resolvePointer returns exists=false → false.
	d := baseDevice()
	d.Properties.Peripherals = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_NilInterfaces_ReturnsFalse(t *testing.T) {
	// Interfaces field is nil → omitted from JSON → key absent → false.
	d := baseDevice()
	d.Properties.Interfaces = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_NilCpus_ReturnsFalse(t *testing.T) {
	// Cpus field is nil → omitted from JSON → key absent → false.
	d := baseDevice()
	d.Properties.Cpus = nil
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 4 — Non-array resolved types
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ResolvedScalarString_ReturnsFalse(t *testing.T) {
	// /vendor resolves to a string scalar.
	// ContainsAny requires an array of objects → must return false.
	d := deviceWithVendor("Acme")
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/vendor",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ResolvedScalarBool_ReturnsFalse(t *testing.T) {
	// /otelCollector resolves to a bool scalar.
	// ContainsAny requires an array of objects → must return false.
	d := baseDevice()
	d.Properties.OtelCollector = pointers.Ptr(true)
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/otelCollector",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ResolvedScalarNumber_ReturnsFalse(t *testing.T) {
	// /cpus/0/cores resolves to float64 after JSON round-trip.
	// ContainsAny requires an array of objects → must return false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus/0/cores",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ResolvedScalarString_ModelNumber_ReturnsFalse(t *testing.T) {
	// /modelNumber resolves to a string scalar → false.
	d := baseDevice()
	d.Properties.ModelNumber = "EF1.234.32"
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/modelNumber",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 5 — Empty array
// ---------------------------------------------------------------------------

func TestHandleContainsAny_EmptyPeripheralsArray_ReturnsFalse(t *testing.T) {
	// Peripherals is an explicitly empty slice (not nil) →
	// nothing to iterate over → false.
	d := baseDevice()
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_EmptyCpusArray_ReturnsFalse(t *testing.T) {
	// Cpus is an explicitly empty slice → nothing to iterate over → false.
	d := baseDevice()
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 6 — Array of scalars (must reject — use In/NotIn instead)
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ArrayOfScalars_SupportedDeploymentTypes_ReturnsFalse(t *testing.T) {
	// /supportedDeploymentTypes is []string (array of scalars).
	// ContainsAny requires an array of objects → must return false.
	d := baseDevice()
	compose := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose
	custom := clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCustom
	d.Properties.SupportedDeploymentTypes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes{
		compose,
		custom,
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/supportedDeploymentTypes",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("compose")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ArrayOfScalars_SupportedRuntimes_ReturnsFalse(t *testing.T) {
	// /supportedRuntimes is []string (array of scalars).
	// ContainsAny requires an array of objects → must return false.
	d := baseDevice()
	oci := clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimesOci
	d.Properties.SupportedRuntimes = &[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes{
		oci,
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/supportedRuntimes",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("oci")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 7 — Single-element array
// ---------------------------------------------------------------------------

func TestHandleContainsAny_SingleElementArray_SingleConditionMatches_ReturnsTrue(t *testing.T) {
	// Only one peripheral present; the single itemSelector expression matches it → true.
	d := baseDevice()
	manufacturer := "NVIDIA"
	model := "RTX 4090"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer, Model: &model},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleElementArray_SingleConditionNoMatch_ReturnsFalse(t *testing.T) {
	// Only one peripheral present (gpu); itemSelector looks for camera → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_SingleElementArray_MultipleConditions_FirstMatches_ReturnsTrue(
	t *testing.T,
) {
	// Only one peripheral (gpu/NVIDIA); itemSelector has two expressions.
	// OR logic: first expression (/type In ["gpu"]) matches → true immediately.
	// Second expression (/manufacturer In ["Logitech"]) would fail but is never needed.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleElementArray_MultipleConditions_NoneMatch_ReturnsFalse(
	t *testing.T,
) {
	// Only one peripheral (gpu/NVIDIA); neither expression matches → false.
	// /type In ["camera"] → false; /manufacturer In ["Logitech"] → false.
	d := baseDevice()
	manufacturer := "NVIDIA"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &manufacturer},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 8 — Single condition OR semantics: peripherals array
// ---------------------------------------------------------------------------

func TestHandleContainsAny_SingleCondition_FirstElementMatches_ReturnsTrue(t *testing.T) {
	// peripherals=[{gpu,NVIDIA,RTX4090},{camera,Logitech,C920},{display,Dell,U2722D}]
	// itemSelector: /type In ["gpu"] → first element matches → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleCondition_SecondElementMatches_ReturnsTrue(t *testing.T) {
	// itemSelector: /type In ["camera"] → second element matches → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("camera")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleCondition_ThirdElementMatches_ReturnsTrue(t *testing.T) {
	// itemSelector: /type In ["display"] → third element matches → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("display")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleCondition_NoElementMatches_ReturnsFalse(t *testing.T) {
	// itemSelector: /type In ["microphone"] → no element has type=microphone → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_SingleCondition_ManufacturerMatches_ReturnsTrue(t *testing.T) {
	// itemSelector: /manufacturer In ["Dell"] → third element matches → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleCondition_ManufacturerNoMatch_ReturnsFalse(t *testing.T) {
	// itemSelector: /manufacturer In ["Samsung"] → no element has that manufacturer → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Samsung")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_SingleCondition_ModelMatches_ReturnsTrue(t *testing.T) {
	// itemSelector: /model In ["C920"] → second element (camera/Logitech) matches → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("C920")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_SingleCondition_ModelNoMatch_ReturnsFalse(t *testing.T) {
	// itemSelector: /model In ["GTX 1080"] → no element has that model → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("GTX 1080")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 9 — Multiple conditions: OR semantics
// ---------------------------------------------------------------------------

// The defining characteristic of ContainsAny:
// For a given element, if ANY single itemSelector expression is satisfied,
// that element passes and ContainsAny returns true.
// This is the direct contrast with ContainsAll, which requires ALL expressions
// to be satisfied by a single element.

func TestHandleContainsAny_MultipleConditions_FirstExpressionMatchesFirstElement_ReturnsTrue(
	t *testing.T,
) {
	// itemSelector: /type In ["gpu"] OR /manufacturer In ["Sony"]
	// First element {gpu,NVIDIA}: /type In ["gpu"] ✓ → true immediately.
	// Second expression is never evaluated for this element.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_SecondExpressionMatchesFirstElement_ReturnsTrue(
	t *testing.T,
) {
	// itemSelector: /type In ["microphone"] OR /manufacturer In ["NVIDIA"]
	// First element {gpu,NVIDIA}: /type In ["microphone"] ✗, /manufacturer In ["NVIDIA"] ✓ → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_ExpressionMatchesSecondElement_ReturnsTrue(
	t *testing.T,
) {
	// itemSelector: /type In ["speaker"] OR /manufacturer In ["Logitech"]
	// First element {gpu,NVIDIA}: both fail.
	// Second element {camera,Logitech}: /manufacturer In ["Logitech"] ✓ → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("speaker")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_ExpressionMatchesThirdElement_ReturnsTrue(
	t *testing.T,
) {
	// itemSelector: /type In ["speaker"] OR /model In ["U2722D"]
	// First element {gpu,NVIDIA,RTX4090}: both fail.
	// Second element {camera,Logitech,C920}: both fail.
	// Third element {display,Dell,U2722D}: /model In ["U2722D"] ✓ → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("speaker")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("U2722D")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_NoExpressionMatchesAnyElement_ReturnsFalse(
	t *testing.T,
) {
	// itemSelector: /type In ["microphone"] OR /manufacturer In ["Sony"]
	// No element has type=microphone or manufacturer=Sony → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_ThreeExpressions_OneMatchesSecondElement_ReturnsTrue(
	t *testing.T,
) {
	// itemSelector: /type In ["speaker"] OR /model In ["GTX 1080"] OR /manufacturer In ["Logitech"]
	// First element: all fail.
	// Second element {camera,Logitech,C920}: /manufacturer In ["Logitech"] ✓ → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("speaker")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("GTX 1080")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_MultipleConditions_ThreeExpressions_NoneMatch_ReturnsFalse(
	t *testing.T,
) {
	// itemSelector: /type In ["microphone"] OR /model In ["GTX 1080"] OR /manufacturer In ["Sony"]
	// No element satisfies any of the three expressions → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
				{Key: "/model", Operator: clModels.In, Values: common.Vals("GTX 1080")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 10 — ContainsAny vs ContainsAll contrast
// ---------------------------------------------------------------------------

// These tests use the same device and expressions to explicitly demonstrate
// the OR vs AND difference between ContainsAny and ContainsAll.

func TestHandleContainsAny_VsContainsAll_ConditionsSplitAcrossElements_ContainsAnyTrue_ContainsAllFalse(
	t *testing.T,
) {
	// itemSelector: /type In ["gpu"] AND/OR /manufacturer In ["Logitech"]
	// gpu→NVIDIA, camera→Logitech: conditions are satisfied by DIFFERENT elements.
	//
	// ContainsAll: requires ONE element to satisfy BOTH → false (no single element has gpu+Logitech).
	// ContainsAny: requires ONE element to satisfy ANY ONE → true
	//              (first element satisfies /type In ["gpu"]).
	d := containsAnyPeripheralDevice()
	e := engine(d)

	meAny := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	meAll := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Logitech")},
			},
		},
	}

	okAny, _ := e.HandleContainsAny(meAny)
	okAll, _ := e.HandleContainsAll(meAll)

	// ContainsAny: true — first element satisfies /type In ["gpu"]
	assert.True(t, okAny)
	// ContainsAll: false — no single element satisfies both conditions
	assert.False(t, okAll)
}

func TestHandleContainsAny_VsContainsAll_AllConditionsMatchSameElement_BothTrue(t *testing.T) {
	// itemSelector: /type In ["gpu"] AND/OR /manufacturer In ["NVIDIA"]
	// First element {gpu,NVIDIA} satisfies both.
	//
	// ContainsAll: true — first element satisfies ALL conditions.
	// ContainsAny: true — first element satisfies ANY condition (/type In ["gpu"]).
	d := containsAnyPeripheralDevice()
	e := engine(d)

	meAny := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	meAll := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	okAny, _ := e.HandleContainsAny(meAny)
	okAll, _ := e.HandleContainsAll(meAll)

	assert.True(t, okAny)
	assert.True(t, okAll)
}

func TestHandleContainsAny_VsContainsAll_NoConditionMatchesAnyElement_BothFalse(t *testing.T) {
	// itemSelector: /type In ["microphone"] AND/OR /manufacturer In ["Sony"]
	// No element satisfies either condition.
	//
	// ContainsAll: false — no element satisfies all conditions.
	// ContainsAny: false — no element satisfies any condition.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	meAny := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	meAll := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAll,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	okAny, _ := e.HandleContainsAny(meAny)
	okAll, _ := e.HandleContainsAll(meAll)

	assert.False(t, okAny)
	assert.False(t, okAll)
}

// ---------------------------------------------------------------------------
// Group 11 — Operator variety inside itemSelector: Exists
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ItemSelector_Exists_FieldPresent_ReturnsTrue(t *testing.T) {
	// itemSelector: /manufacturer Exists (OR logic)
	// First element has manufacturer set → Exists returns true → ContainsAny true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_Exists_FieldAbsentInAllElements_ReturnsFalse(t *testing.T) {
	// All peripherals have no Model set → /model Exists fails for every element → false.
	d := baseDevice()
	m1 := "NVIDIA"
	m2 := "Logitech"
	// Model intentionally omitted on all peripherals.
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &m1},
		{Type: clModels.Camera, Manufacturer: &m2},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ItemSelector_Exists_FieldPresentOnSecondElement_ReturnsTrue(
	t *testing.T,
) {
	// First peripheral has no model; second has model set.
	// OR logic: /model Exists fails for first element, passes for second → true.
	d := baseDevice()
	m1 := "NVIDIA"
	m2 := "Logitech"
	model2 := "C920"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &m1},
		{Type: clModels.Camera, Manufacturer: &m2, Model: &model2},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.Exists},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 12 — Operator variety inside itemSelector: DoesNotExist
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ItemSelector_DoesNotExist_FieldAbsentInFirstElement_ReturnsTrue(
	t *testing.T,
) {
	// First peripheral has no model → /model DoesNotExist passes → true immediately.
	d := baseDevice()
	m1 := "NVIDIA"
	m2 := "Logitech"
	model2 := "C920"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Gpu, Manufacturer: &m1}, // no model
		{Type: clModels.Camera, Manufacturer: &m2, Model: &model2},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_DoesNotExist_FieldPresentInAllElements_ReturnsFalse(
	t *testing.T,
) {
	// All peripherals have manufacturer set → /manufacturer DoesNotExist fails for all → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.DoesNotExist},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 13 — Operator variety inside itemSelector: NotIn
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ItemSelector_NotIn_FirstElementNotInValues_ReturnsTrue(t *testing.T) {
	// itemSelector: /type NotIn ["microphone","speaker"]
	// First element type=gpu is not in that list → NotIn passes → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.NotIn,
					Values:   common.Vals("microphone", "speaker"),
				},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_NotIn_AllElementsInValues_ReturnsFalse(t *testing.T) {
	// itemSelector: /type NotIn ["gpu","camera","display"]
	// All three elements have types in that list → NotIn fails for all → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{
					Key:      "/type",
					Operator: clModels.NotIn,
					Values:   common.Vals("gpu", "camera", "display"),
				},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 14 — Operator variety inside itemSelector: Gt
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ItemSelector_Gt_SecondElementCoresGreaterThanThreshold_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Gt [6]
	// First element cores=4: 4 > 6 → false.
	// Second element cores=8: 8 > 6 → true → ContainsAny true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_Gt_FirstElementCoresGreaterThanThreshold_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Gt [3]
	// First element cores=4: 4 > 3 → true → ContainsAny true immediately.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(3))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_Gt_NoCoresGreaterThanThreshold_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Gt [10] → neither element has cores > 10 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(10))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 15 — Operator variety inside itemSelector: Lt
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ItemSelector_Lt_FirstElementCoresLessThanThreshold_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Lt [5]
	// First element cores=4: 4 < 5 → true → ContainsAny true immediately.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(5))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_Lt_NoCoresLessThanThreshold_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Lt [3] → neither element has cores < 3 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(3))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ItemSelector_GtOrIn_ORSemantics_FirstExpressionMatches_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Gt [10] OR /architecture In ["amd64"]
	// First element: /cores Gt [10] → 4 > 10 ✗; /architecture In ["amd64"] → ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(10))},
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ItemSelector_GtAndLt_ORSemantics_NeitherMatches_ReturnsFalse(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /cores Gt [10] OR /cores Lt [2]
	// No element has cores > 10 or cores < 2 → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(10))},
				{Key: "/cores", Operator: clModels.Lt, Values: common.Vals(float64(2))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 16 — Cpus array
// ---------------------------------------------------------------------------

func TestHandleContainsAny_CpusArray_ArchAmd64_Matches_ReturnsTrue(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /architecture In ["amd64"] → first element matches → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_CpusArray_ArchArm64_Matches_ReturnsTrue(t *testing.T) {
	// itemSelector: /architecture In ["arm64"] → second element matches → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_CpusArray_ArchRiscv64_NoMatch_ReturnsFalse(t *testing.T) {
	// itemSelector: /architecture In ["riscv64"] → neither element matches → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_CpusArray_MultipleExpressions_OR_FirstArchMatches_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /architecture In ["amd64"] OR /cores Gt [100]
	// First element: /architecture In ["amd64"] ✓ → true immediately.
	// /cores Gt [100] would fail but is never needed.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(100))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_CpusArray_MultipleExpressions_OR_SecondArchMatches_ReturnsTrue(
	t *testing.T,
) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /architecture In ["riscv64"] OR /cores Gt [6]
	// First element: /architecture In ["riscv64"] ✗; /cores Gt [6] → 4 > 6 ✗.
	// Second element: /architecture In ["riscv64"] ✗; /cores Gt [6] → 8 > 6 ✓ → true.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(6))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_CpusArray_MultipleExpressions_OR_NoneMatch_ReturnsFalse(t *testing.T) {
	// cpuDevice: [{amd64,4},{arm64,8}]
	// itemSelector: /architecture In ["riscv64"] OR /cores Gt [100]
	// No element satisfies either expression → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
				{Key: "/cores", Operator: clModels.Gt, Values: common.Vals(float64(100))},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 17 — Interfaces array
// ---------------------------------------------------------------------------

func TestHandleContainsAny_InterfacesArray_EthernetMatches_ReturnsTrue(t *testing.T) {
	// /interfaces=[{ethernet},{wifi}]
	// itemSelector: /type In ["ethernet"] → first element matches → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_InterfacesArray_WifiMatches_ReturnsTrue(t *testing.T) {
	// itemSelector: /type In ["wifi"] → second element matches → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("wifi")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_InterfacesArray_BluetoothNoMatch_ReturnsFalse(t *testing.T) {
	// itemSelector: /type In ["bluetooth"] → neither element has type=bluetooth → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("bluetooth")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_InterfacesArray_MultipleExpressions_OR_SecondMatches_ReturnsTrue(
	t *testing.T,
) {
	// /interfaces=[{ethernet},{wifi}]
	// itemSelector: /type In ["bluetooth"] OR /type In ["wifi"]
	// First element: both fail.
	// Second element: /type In ["bluetooth"] ✗; /type In ["wifi"] ✓ → true.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
		{Type: clModels.Wifi},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/interfaces",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("bluetooth")},
				{Key: "/type", Operator: clModels.In, Values: common.Vals("wifi")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 18 — Invalid pointer format
// ---------------------------------------------------------------------------

func TestHandleContainsAny_InvalidPointer_MissingLeadingSlash_Peripherals_ReturnsFalse(
	t *testing.T,
) {
	// RFC 6901: pointer must start with '/'.
	// "peripherals" without leading slash → resolvePointer returns error → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "peripherals", // missing leading '/'
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_InvalidPointer_MissingLeadingSlash_Cpus_ReturnsFalse(t *testing.T) {
	// "cpus" without leading slash → resolvePointer returns error → false.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "cpus", // missing leading '/'
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_InvalidPointer_MissingLeadingSlash_Interfaces_ReturnsFalse(
	t *testing.T,
) {
	// "interfaces" without leading slash → resolvePointer returns error → false.
	d := baseDevice()
	d.Properties.Interfaces = &[]clModels.DeviceCommunicationInterface{
		{Type: clModels.Ethernet},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "interfaces", // missing leading '/'
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("ethernet")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 19 — Case sensitivity in itemSelector key values
// ---------------------------------------------------------------------------

func TestHandleContainsAny_CaseSensitivity_TypeUpperCase_ReturnsFalse(t *testing.T) {
	// Spec: string comparisons MUST be exact and case-sensitive.
	// "GPU" != "gpu" → no element matches → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("GPU")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_CaseSensitivity_ManufacturerLowerCase_ReturnsFalse(t *testing.T) {
	// "nvidia" != "NVIDIA" → no element matches → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("nvidia")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_CaseSensitivity_ExactMatch_ReturnsTrue(t *testing.T) {
	// "NVIDIA" matches exactly → true.
	// Guards against any unintended case normalisation in the implementation.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("NVIDIA")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 20 — Idempotency: repeated calls on the same engine
// ---------------------------------------------------------------------------

func TestHandleContainsAny_RepeatedCalls_SameResult_ReturnsTrue(t *testing.T) {
	// Calling HandleContainsAny twice on the same engine with a matching
	// expression must produce identical true results.
	// Guards against accidental mutation of internal state between calls.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me)
	ok2, reason2 := e.HandleContainsAny(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleContainsAny_RepeatedCalls_SameResult_ReturnsFalse(t *testing.T) {
	// Calling HandleContainsAny twice on the same engine with a non-matching
	// expression must produce identical false results.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me)
	ok2, reason2 := e.HandleContainsAny(me)

	require.False(t, ok1)
	require.False(t, ok2)
	assert.Equal(t, reason1, reason2)
}

func TestHandleContainsAny_RepeatedCalls_CpusArray_SameResult_ReturnsTrue(t *testing.T) {
	// Repeated calls on cpus array must produce consistent results.
	d := cpuDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("arm64")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me)
	ok2, reason2 := e.HandleContainsAny(me)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, reason1, reason2)
}

// ---------------------------------------------------------------------------
// Group 21 — Two independent ContainsAny expressions do not interfere
// ---------------------------------------------------------------------------

func TestHandleContainsAny_TwoIndependentExpressions_BothTrue_DoNotInterfere(t *testing.T) {
	// Evaluating two different ContainsAny expressions on the same engine
	// must not corrupt state between calls.
	// expr1: /peripherals → /type In ["gpu"]          → true
	// expr2: /peripherals → /manufacturer In ["Dell"] → true (display element)
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Dell")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me1)
	ok2, reason2 := e.HandleContainsAny(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.True(t, ok2)
	assert.Empty(t, reason2)
}

func TestHandleContainsAny_TwoIndependentExpressions_FirstTrueSecondFalse_DoNotInterfere(
	t *testing.T,
) {
	// expr1: /peripherals → /type In ["gpu"]          → true
	// expr2: /peripherals → /manufacturer In ["Sony"] → false
	// Results must be independent.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/manufacturer", Operator: clModels.In, Values: common.Vals("Sony")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me1)
	ok2, reason2 := e.HandleContainsAny(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.False(t, ok2)
	assert.NotEmpty(t, reason2)
}

func TestHandleContainsAny_TwoIndependentExpressions_DifferentArrays_BothTrue_DoNotInterfere(
	t *testing.T,
) {
	// expr1: /peripherals → /type In ["gpu"]            → true
	// expr2: /cpus        → /architecture In ["amd64"]  → true
	// Different arrays on the same engine must not interfere.
	d := containsAnyPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me1)
	ok2, reason2 := e.HandleContainsAny(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.True(t, ok2)
	assert.Empty(t, reason2)
}

func TestHandleContainsAny_TwoIndependentExpressions_DifferentArrays_FirstTrueSecondFalse(
	t *testing.T,
) {
	// expr1: /peripherals → /type In ["gpu"]              → true
	// expr2: /cpus        → /architecture In ["riscv64"]  → false
	d := containsAnyPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	me1 := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	me2 := clModels.MatchExpression{
		Key:      "/cpus",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/architecture", Operator: clModels.In, Values: common.Vals("riscv64")},
			},
		},
	}

	ok1, reason1 := e.HandleContainsAny(me1)
	ok2, reason2 := e.HandleContainsAny(me2)

	assert.True(t, ok1)
	assert.Empty(t, reason1)
	assert.False(t, ok2)
	assert.NotEmpty(t, reason2)
}

// ---------------------------------------------------------------------------
// Group 22 — Evaluate integration: ContainsAny inside a full Selector
// ---------------------------------------------------------------------------

func TestHandleContainsAny_ViaEvaluate_SingleContainsAnyExpression_ReturnsTrue(t *testing.T) {
	// Evaluate a Selector containing a single ContainsAny expression.
	// /peripherals ContainsAny {/type In ["gpu"]} → true.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAny,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ViaEvaluate_ContainsAnyAndInExpression_BothTrue_ReturnsTrue(
	t *testing.T,
) {
	// Selector with two expressions ANDed together:
	// 1. /vendor In ["Acme Corp"]                              → true
	// 2. /peripherals ContainsAny {/type In ["gpu"]}           → true
	// Both true → Evaluate returns true.
	d := containsAnyPeripheralDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAny,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleContainsAny_ViaEvaluate_ContainsAnyAndInExpression_ContainsAnyFails_ReturnsFalse(
	t *testing.T,
) {
	// Selector with two expressions ANDed together:
	// 1. /vendor In ["Acme Corp"]                                    → true
	// 2. /peripherals ContainsAny {/type In ["microphone"]}          → false
	// Second expression fails → Evaluate returns false.
	d := containsAnyPeripheralDevice()
	d.Properties.Vendor = "Acme Corp"
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/vendor",
				Operator: clModels.In,
				Values:   common.Vals("Acme Corp"),
			},
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAny,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("microphone")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_ViaEvaluate_TwoContainsAnyExpressions_BothTrue_ReturnsTrue(
	t *testing.T,
) {
	// Selector with two ContainsAny expressions ANDed:
	// 1. /peripherals ContainsAny {/type In ["gpu"]}           → true
	// 2. /cpus        ContainsAny {/architecture In ["amd64"]} → true
	d := containsAnyPeripheralDevice()
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	d.Properties.Cpus = &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: &amd64, Cores: float32(4)},
	}
	e := engine(d)

	selector := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{
				Key:      "/peripherals",
				Operator: clModels.ContainsAny,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
					},
				},
			},
			{
				Key:      "/cpus",
				Operator: clModels.ContainsAny,
				ItemSelector: &clModels.Selector{
					MatchExpressions: []clModels.MatchExpression{
						{Key: "/architecture", Operator: clModels.In, Values: common.Vals("amd64")},
					},
				},
			},
		},
	}

	ok, reason := e.Evaluate(selector)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ---------------------------------------------------------------------------
// Group 23 — Edge cases
// ---------------------------------------------------------------------------

func TestHandleContainsAny_OutOfBoundsIndexInKey_ReturnsFalse(t *testing.T) {
	// /peripherals/5 → index 5 does not exist in a 3-element array →
	// resolvePointer returns exists=false → false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals/5",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_EmptyStringKey_ReturnsWholePropertiesNotArray_ReturnsFalse(
	t *testing.T,
) {
	// RFC 6901: empty string refers to the whole document (properties struct).
	// The whole properties struct is not an array of objects →
	// ContainsAny must return false.
	d := containsAnyPeripheralDevice()
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/type", Operator: clModels.In, Values: common.Vals("gpu")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleContainsAny_LargePeripheralArray_MatchOnLastElement_ReturnsTrue(t *testing.T) {
	// Array with many peripherals; only the last one satisfies the condition.
	// Verifies the implementation iterates through all elements before giving up.
	d := baseDevice()
	m1 := "Vendor1"
	m2 := "Vendor2"
	m3 := "Vendor3"
	targetM := "TargetVendor"
	targetModel := "TargetModel"
	d.Properties.Peripherals = &[]clModels.DevicePeripheral{
		{Type: clModels.Camera, Manufacturer: &m1},
		{Type: clModels.Display, Manufacturer: &m2},
		{Type: clModels.Microphone, Manufacturer: &m3},
		{Type: clModels.Gpu, Manufacturer: &targetM, Model: &targetModel},
	}
	e := engine(d)

	me := clModels.MatchExpression{
		Key:      "/peripherals",
		Operator: clModels.ContainsAny,
		ItemSelector: &clModels.Selector{
			MatchExpressions: []clModels.MatchExpression{
				{Key: "/model", Operator: clModels.In, Values: common.Vals("TargetModel")},
			},
		},
	}

	ok, reason := e.HandleContainsAny(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}
