package main

import (
	"context"

	"github.com/margo/dev-repo/poc/device/agent/database"
	workloads "github.com/margo/dev-repo/shared-lib/workloads"
	"go.uber.org/zap"
)

type WorkloadWatcher interface {
	Start() error
	Stop() error
}

type workloadWatcher struct {
	ctx                 context.Context
	operationStopper    context.CancelFunc
	log                 *zap.SugaredLogger
	database            database.AgentDatabase
	helmClient          *workloads.HelmClient
	dockerComposeClient *workloads.DockerComposeClient
}

// NewWorkloadWatcher creates a new WorkloadManager instance
func NewWorkloadWatcher(ctx context.Context, log *zap.SugaredLogger, database database.AgentDatabase, helmClient *workloads.HelmClient, dockerComposeClient *workloads.DockerComposeClient) *workloadWatcher {
	localCtx, localCanceller := context.WithCancel(ctx)
	return &workloadWatcher{
		log:                 log,
		ctx:                 localCtx,
		operationStopper:    localCanceller,
		database:            database,
		helmClient:          helmClient,
		dockerComposeClient: dockerComposeClient,
	}
}

func (watcher *workloadWatcher) Start() error {
	return nil
}

func (watcher *workloadWatcher) Stop() error {
	return nil
}
