package compose

import (
	"sort"
	"time"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v5/pkg/api"
)

type ContainerState string

const (
	StateCreated    ContainerState = "created"    // StateCreated indicates the container is created, but not (yet) started.
	StateRunning    ContainerState = "running"    // StateRunning indicates that the container is running.
	StatePaused     ContainerState = "paused"     // StatePaused indicates that the container's current state is paused.
	StateRestarting ContainerState = "restarting" // StateRestarting indicates that the container is currently restarting.
	StateRemoving   ContainerState = "removing"   // StateRemoving indicates that the container is being removed.
	StateExited     ContainerState = "exited"     // StateExited indicates that the container exited.
	StateDead       ContainerState = "dead"       // StateDead indicates that the container failed to be deleted. Containers in this state are attempted to be cleaned up when the daemon restarts.
)

// HealthStatus is a string representation of the container's health.
type HealthStatus string

// Health states
const (
	NoHealthcheck HealthStatus = "none"      // Indicates there is no healthcheck
	Starting      HealthStatus = "starting"  // Starting indicates that the container is not yet ready
	Healthy       HealthStatus = "healthy"   // Healthy indicates that the container is running correctly
	Unhealthy     HealthStatus = "unhealthy" // Unhealthy indicates that the container has a problem
)

type Project struct {
	Name             string
	WorkingDir       string
	Services         types.Services
	Networks         types.Networks
	Volumes          types.Volumes
	Secrets          types.Secrets
	Configs          types.Configs
	Models           types.Models
	Extensions       types.Extensions
	ComposeFiles     []string
	Environment      types.Mapping
	DisabledServices types.Services
	Profiles         []string
}

type ProjectLoadOptions struct {
	ProjectName   string
	ConfigPaths   []string
	WorkingDir    string
	EnvFiles      []string
	Profiles      []string
	Services      []string
	Environment   map[string]string
	Offline       bool
	All           bool
	Compatibility bool
}

type DownOptions struct {
	// RemoveOrphans will cleanup containers that are not declared on the compose model but own the same labels
	RemoveOrphans bool
	// Project is the compose project used to define this app. Might be nil if user ran `down` just with project name
	Project Project
	// Timeout override container stop timeout
	Timeout *time.Duration
	// Images remove image used by services. 'all': Remove all images. 'local': Remove only images that don't have a tag
	Images string
	// Volumes remove volumes, both declared in the `volumes` section and anonymous ones
	Volumes bool
	// Services passed in the command line to be stopped
	Services []string
}
type PushOptions struct {
	Quiet          bool
	IgnoreFailures bool
	ImageMandatory bool
}

type PullOptions struct {
	Quiet           bool
	IgnoreFailures  bool
	IgnoreBuildable bool
}

// PsOptions group options of the Ps API
type PsOptions struct {
	Project  *Project
	All      bool
	Services []string
}

type ContainerSummary struct {
	ID           string
	Name         string
	Names        []string
	Image        string
	Command      string
	Project      string
	Service      string
	Created      int64
	State        ContainerState
	Status       string
	Health       HealthStatus
	ExitCode     int
	Publishers   PortPublishers
	Labels       map[string]string
	SizeRw       int64 `json:",omitempty"`
	SizeRootFs   int64 `json:",omitempty"`
	Mounts       []string
	Networks     []string
	LocalVolumes int
}

// PortPublishers is a slice of PortPublisher
type PortPublishers []PortPublisher

// PortPublisher hold status about published port
type PortPublisher struct {
	URL           string
	TargetPort    int
	PublishedPort int
	Protocol      string
}

// RemoveOptions group options of the Remove API
type RemoveOptions struct {
	// Project is the compose project used to define this app. Might be nil if user ran command just with project name
	Project *Project
	// Stop option passed in the command line
	Stop bool
	// Volumes remove anonymous volumes
	Volumes bool
	// Force don't ask to confirm removal
	Force bool
	// Services passed in the command line to be removed
	Services []string
}

// RestartOptions group options of the Restart API
type RestartOptions struct {
	// Project is the compose project used to define this app. Might be nil if user ran command just with project name
	Project *Project
	// Timeout override container restart timeout
	Timeout *time.Duration
	// Services passed in the command line to be restarted
	Services []string
	// NoDeps ignores services dependencies
	NoDeps bool
}

func toComposeRemoveOptions(opts RemoveOptions) api.RemoveOptions {
	removeOptions := api.RemoveOptions{
		Project: toComposeProject(opts.Project),
		Stop:    opts.Stop,
		Volumes: opts.Volumes,
		Force:   opts.Force,
	}

	if opts.Services != nil {
		removeOptions.Services = opts.Services
	}

	return removeOptions
}

func toComposeRestartOptions(opts RestartOptions) api.RestartOptions {
	restartOptions := api.RestartOptions{
		Project: toComposeProject(opts.Project),
		NoDeps:  opts.NoDeps,
	}
	if opts.Timeout != nil {
		restartOptions.Timeout = opts.Timeout
	}
	if opts.Services != nil {
		restartOptions.Services = opts.Services
	}
	return restartOptions
}

func fromApiContainerSummaryToSummary(cs api.ContainerSummary) ContainerSummary {
	summary := ContainerSummary{
		ID:           cs.ID,
		Name:         cs.Name,
		Names:        cs.Names,
		Image:        cs.Image,
		Command:      cs.Command,
		Project:      cs.Project,
		Service:      cs.Service,
		Created:      cs.Created,
		State:        ContainerState(cs.State),
		Status:       cs.Status,
		Health:       HealthStatus(cs.Health),
		ExitCode:     cs.ExitCode,
		Labels:       cs.Labels,
		SizeRw:       cs.SizeRw,
		SizeRootFs:   cs.SizeRootFs,
		Mounts:       cs.Mounts,
		Networks:     cs.Networks,
		LocalVolumes: cs.LocalVolumes,
	}

	if cs.Publishers != nil {
		summary.Publishers = make(PortPublishers, len(cs.Publishers))
		for i, publisher := range cs.Publishers {
			summary.Publishers[i] = PortPublisher{
				URL:           publisher.URL,
				TargetPort:    publisher.TargetPort,
				PublishedPort: publisher.PublishedPort,
				Protocol:      publisher.Protocol,
			}
		}
	}

	return summary
}

func toComposePsOptions(opts PsOptions) api.PsOptions {
	psOptions := api.PsOptions{
		Project:  toComposeProject(opts.Project),
		All:      opts.All,
		Services: opts.Services,
	}

	if opts.Services != nil {
		psOptions.Services = opts.Services
	}

	return psOptions
}

func toComposePullOptions(opts PullOptions) api.PullOptions {
	return api.PullOptions{
		Quiet:           opts.Quiet,
		IgnoreFailures:  opts.IgnoreFailures,
		IgnoreBuildable: opts.IgnoreBuildable,
	}
}

func toComposeProjectLoadOptions(opts ProjectLoadOptions) api.ProjectLoadOptions {
	projectOptions := api.ProjectLoadOptions{
		ProjectName:   opts.ProjectName,
		ConfigPaths:   opts.ConfigPaths,
		WorkingDir:    opts.WorkingDir,
		EnvFiles:      opts.EnvFiles,
		Profiles:      opts.Profiles,
		Services:      opts.Services,
		Offline:       opts.Offline,
		All:           opts.All,
		Compatibility: opts.Compatibility,
	}

	if len(opts.Environment) > 0 {
		env := make([]string, 0, len(opts.Environment))
		for key, value := range opts.Environment {
			env = append(env, key+"="+value)
		}
		sort.Strings(env)
		projectOptions.ProjectOptionsFns = []cli.ProjectOptionsFn{
			cli.WithEnv(env),
		}
	}

	return projectOptions
}

func toComposeProject(project *Project) *types.Project {
	if project == nil {
		return nil
	}

	return &types.Project{
		Name:             project.Name,
		WorkingDir:       project.WorkingDir,
		Services:         project.Services,
		Networks:         project.Networks,
		Volumes:          project.Volumes,
		Secrets:          project.Secrets,
		Configs:          project.Configs,
		Models:           project.Models,
		Extensions:       project.Extensions,
		ComposeFiles:     project.ComposeFiles,
		Environment:      project.Environment,
		DisabledServices: project.DisabledServices,
		Profiles:         project.Profiles,
	}
}

func fromComposeProject(project *types.Project) *Project {
	if project == nil {
		return nil
	}

	return &Project{
		Name:             project.Name,
		WorkingDir:       project.WorkingDir,
		Services:         project.Services,
		Networks:         project.Networks,
		Volumes:          project.Volumes,
		Secrets:          project.Secrets,
		Configs:          project.Configs,
		Models:           project.Models,
		Extensions:       project.Extensions,
		ComposeFiles:     project.ComposeFiles,
		Environment:      project.Environment,
		DisabledServices: project.DisabledServices,
		Profiles:         project.Profiles,
	}
}

func toComposeDownOptions(d DownOptions) api.DownOptions {
	downOption := api.DownOptions{
		RemoveOrphans: d.RemoveOrphans,
		Project:       toComposeProject(&d.Project),
		Images:        d.Images,
		Volumes:       d.Volumes,
	}

	if d.Services != nil {
		downOption.Services = d.Services
	}

	if d.Timeout != nil {
		downOption.Timeout = d.Timeout
	}
	return downOption
}

func toComposePushOptions(options PushOptions) api.PushOptions {
	return api.PushOptions{
		Quiet:          options.Quiet,
		IgnoreFailures: options.IgnoreFailures,
		ImageMandatory: options.ImageMandatory,
	}
}
