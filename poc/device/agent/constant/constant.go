package constant

import "strings"

type DeploymentRecordChangeType string

const (
	DeploymentChangeTypeRecordAdded           DeploymentRecordChangeType = "RECORD-ADDED"
	DeploymentChangeTypeRecordDeleted         DeploymentRecordChangeType = "RECORD-DELETED"
	DeploymentChangeTypeComponentPhaseChanged DeploymentRecordChangeType = "COMPONENT-PHASE-CHANGED"
	DeploymentChangeTypeDesiredStateAdded     DeploymentRecordChangeType = "DESIRED-STATE-ADDED"
	DeploymentChangeTypeCurrentStateAdded     DeploymentRecordChangeType = "CURRENT-STATE-ADDED"
)

type Phase string

const (
	PhaseDeploying Phase = "deploying"
	PhaseRunning   Phase = "running"
	PhaseFailed    Phase = "failed"
	PhaseRemoving  Phase = "removing"
	PhaseRemoved   Phase = "removed"
	PhasePending   Phase = "pending"
)

func (p Phase) ToLower() Phase {
	return Phase(strings.ToLower(string(p)))
}

type AppDeploymentStateState string

const (
	AppDeploymentStateStatePending = "PENDING"
)
