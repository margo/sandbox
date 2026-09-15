package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThatConfigIsLoadedFromDisk(t *testing.T) {

	manifest, err := LoadCapabilities("../config/capabilities.json")
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, manifest.Labels)
	k := *manifest.Labels
	str, err := k["northstarida.com/hypervisor"].AsDeviceCapabilitiesManifestLabels0()
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, "hyper-v", str, "hypervisor value should be equal")
}
