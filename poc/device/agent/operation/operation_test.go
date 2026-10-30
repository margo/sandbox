package operation_test

import (
	"os"
	"testing"

	"github.com/margo/sandbox/poc/device/agent/operation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerComposeClient_Connection(t *testing.T) {
	socketPath := "unix:///var/run/docker.sock"

	workingDir := "data/composeFiles"
	params := operation.DockerConnectivityParams{
		ViaSocket: &operation.DockerSocketParams{
			SocketPath: socketPath,
		},
	}

	client, err := operation.NewDockerComposeClient(params, workingDir)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verify working directory was created
	info, err := os.Stat(workingDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
