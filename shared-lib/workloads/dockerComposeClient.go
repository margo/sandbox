package workloads

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
)

// ComposeClient defines the interface for Docker Compose operations.
type ComposeClient interface {
	Push(ctx context.Context, project *Project, options PushOptions) error
	Pull(ctx context.Context, project *Project, options PullOptions) error
	Ps(ctx context.Context, projectName string, options PsOptions) ([]ContainerSummary, error)
	Restart(ctx context.Context, projectName string, options RestartOptions) error
	Up(ctx context.Context, project *Project) error
	Down(ctx context.Context, projectName string, options DownOptions) error
	Remove(ctx context.Context, projectName string, options RemoveOptions) error
	LoadProject(ctx context.Context, options ProjectLoadOptions) (*Project, error)
}

type DockerComposeClient struct {
	service api.Compose
}

// NewComposeService wraps compose.NewComposeService to instantiate an api.Compose service from a Docker CLI instance.
func NewComposeService(dockerCli command.Cli, opts ...compose.Option) (ComposeClient, error) {
	service, err := compose.NewComposeService(dockerCli, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose service: %w", err)
	}
	return &DockerComposeClient{
		service: service,
	}, nil
}

func (c *DockerComposeClient) Push(ctx context.Context, project *Project, options PushOptions) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}
	proj := toComposeProject(project)
	opts := toComposePushOptions(options)
	if err := c.service.Push(ctx, proj, opts); err != nil {
		return fmt.Errorf("failed to push compose: %w", err)
	}
	return nil
}

func (c *DockerComposeClient) Pull(ctx context.Context, project *Project, options PullOptions) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}
	proj := toComposeProject(project)
	opts := toComposePullOptions(options)
	if err := c.service.Pull(ctx, proj, opts); err != nil {
		return fmt.Errorf("failed to pull compose: %w", err)
	}
	return nil
}

func (c *DockerComposeClient) Ps(ctx context.Context, projectName string, options PsOptions) ([]ContainerSummary, error) {
	if c.service == nil {
		return nil, fmt.Errorf("compose service is not initialized")
	}

	opts := toComposePsOptions(options)
	containerSummary, err := c.service.Ps(ctx, projectName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get compose status: %w", err)
	}
	var summary []ContainerSummary
	for _, cs := range containerSummary {
		summary = append(summary, fromApiContainerSummaryToSummary(cs))
	}
	return summary, nil
}

// Restart restarts containers
func (c *DockerComposeClient) Restart(ctx context.Context, projectName string, options RestartOptions) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}
	opts := toComposeRestartOptions(options)
	if err := c.service.Restart(ctx, projectName, opts); err != nil {
		return fmt.Errorf("failed to restart compose: %w", err)
	}
	return nil
}

// Up executes the equivalent to a `compose up`
func (c *DockerComposeClient) Up(ctx context.Context, project *Project) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}
	if project == nil {
		return fmt.Errorf("compose project is nil")
	}

	proj := toComposeProject(project)
	err := c.service.Up(ctx, proj, api.UpOptions{
		Create: api.CreateOptions{
			Recreate: api.RecreateForce,
		},
		Start: api.StartOptions{
			Project: proj,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to up service: %w", err)
	}
	return nil
}

// Down executes the equivalent to a `compose down`
func (c *DockerComposeClient) Down(ctx context.Context, projectName string, options DownOptions) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}
	downOption := toComposeDownOptions(options)
	if err := c.service.Down(ctx, projectName, downOption); err != nil {
		return fmt.Errorf("failed to compose down project: %w", err)
	}
	return nil
}

// Remove executes the equivalent to a `compose rm`
func (c *DockerComposeClient) Remove(ctx context.Context, projectName string, options RemoveOptions) error {
	if c.service == nil {
		return fmt.Errorf("compose service is not initialized")
	}

	removeOptions := toComposeRemoveOptions(options)
	if err := c.service.Remove(ctx, projectName, removeOptions); err != nil {
		return fmt.Errorf("failed to remove compose: %w", err)
	}
	return nil
}

// LoadProject loads and validates a Compose project from configuration files.
func (c *DockerComposeClient) LoadProject(ctx context.Context, options ProjectLoadOptions) (*Project, error) {
	if c.service == nil {
		return nil, fmt.Errorf("compose service is not initialized")
	}

	project, err := c.service.LoadProject(ctx, toComposeProjectLoadOptions(options))
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project: %w", err)
	}
	return fromComposeProject(project), nil
}
