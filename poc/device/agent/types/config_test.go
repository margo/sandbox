package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThatConfigIsLoadedFromDisk(t *testing.T) {
	manifest, err := LoadDeviceProfile("../config/profiles.json")
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, manifest.Characteristics, "characteristics should not be nil")
}
