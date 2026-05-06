//go:build !compose
// +build !compose

package workloads

import (
	"context"
	"fmt"
	"time"
)

type DockerComposeClient struct {
}

type DockerConnectionViaHttp struct {
	Protocol   string
	Host       string
	Port       uint16
	CaCertPath string
	CertPath   string
	KeyPath    string
}

type DockerConnectionViaSocket struct {
	SocketPath string
}

type DockerConnectivityParams struct {
	ViaHttp   *DockerConnectionViaHttp
	ViaSocket *DockerConnectionViaSocket
}

type ComposeStatus struct {
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Services  []ServiceStatus `json:"services"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ServiceStatus struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Image       string   `json:"image"`
	Ports       []string `json:"ports"`
	ContainerID string   `json:"container_id"`
	Health      string   `json:"health"`
}

func NewDockerComposeClient(
	params DockerConnectivityParams,
	workingDir string,
) (*DockerComposeClient, error) {
	return nil, fmt.Errorf("docker compose support not compiled in (use -tags=compose to enable)")
}

func (c *DockerComposeClient) DeployCompose(
	ctx context.Context,
	projectName string,
	composeFile string,
	envVars map[string]string,
) error {
	return fmt.Errorf("docker compose support not compiled in")
}

func (c *DockerComposeClient) RemoveCompose(
	ctx context.Context,
	projectName string,
) error {
	return fmt.Errorf("docker compose support not compiled in")
}

func (c *DockerComposeClient) GetComposeStatus(
	ctx context.Context,
	projectName string,
) (*ComposeStatus, error) {
	return nil, fmt.Errorf("docker compose support not compiled in")
}

func (c *DockerComposeClient) FetchComposeFileFromURL(
	ctx context.Context,
	url string,
	filename string,
) (string, error) {
	return "", fmt.Errorf("docker compose support not compiled in (use -tags=compose to enable)")
}
