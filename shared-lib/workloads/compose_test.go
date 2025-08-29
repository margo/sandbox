package workloads

import (
	"context"
	"log"
	"testing"
)

func TestFetchComposeFileFromURL(t *testing.T) {
	composeClient, err := NewDockerComposeClient(DockerConnectivityParams{
		ViaSocket: &DockerConnectionViaSocket{
			SocketPath: "unix:///var/run/docker.sock",
		},
	}, "testData/composeFiles")
	if err != nil {
		t.Fatal(err)
	}

	url := "https://github.com/docker/awesome-compose/blob/master/nextcloud-redis-mariadb/compose.yaml"
	data, err := composeClient.FetchComposeFileFromURL(context.Background(), url, "file1.compose.yaml")

	if err != nil {
		t.Fatal(err)
	}

	log.Println("compose file content", string(data))
}
