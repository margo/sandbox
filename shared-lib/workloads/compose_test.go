package workloads

import (
	"context"
	"log"
	"os"
	"testing"
)

func TestFetchComposeFileFromURL(t *testing.T) {
	// Skip this test when Docker socket is not available in the environment
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("docker socket not available; skipping environment-dependent test")
	}

	composeClient, err := NewDockerComposeClient(DockerConnectivityParams{
		ViaSocket: &DockerConnectionViaSocket{
			SocketPath: "unix:///var/run/docker.sock",
		},
	}, "testData/composeFiles")
	if err != nil {
		t.Skipf("docker not available or cannot initialize client: %v", err)
	}

	url := "https://github.com/docker/awesome-compose/blob/master/nextcloud-redis-mariadb/compose.yaml"
	data, err := composeClient.FetchComposeFileFromURL(
		context.Background(),
		url,
		"file1.compose.yaml",
	)

	if err != nil {
		t.Fatal(err)
	}

	log.Println("compose file content", (data))
}
