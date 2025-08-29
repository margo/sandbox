package types

import (
	"fmt"
)

type AgentComponent string

const (
	AgentComponentDatabase   AgentComponent = "database"
	AgentComponentDeployment AgentComponent = "deployment"
	AgentComponentMonitoring AgentComponent = "monitoring"
	AgentComponentOnboarding AgentComponent = "onboarding"
	AgentComponentConfig     AgentComponent = "config"
)

type AgentOperation string

const (
	AgentOperationReadingConfig            AgentOperation = "reading-config"
	AgentOperationReadingValidatingConfig  AgentOperation = "validating-config"
	AgentOperationReadingCapabilities      AgentOperation = "reading-capabilitiess"
	AgentOperationReadingValidating        AgentOperation = "validating-capabilitiess"
	AgentOperationOnboarding               AgentOperation = "onboarding"
	AgentOperationTokenAuthentication      AgentOperation = "token-authentication"
	AgentOperationDeployingApp             AgentOperation = "deploying-app"
	AgentOperationRemovingApp              AgentOperation = "removing-app"
	AgentOperationUpdatingApp              AgentOperation = "updating-app"
	AgentOperationSyncingStateWithWfm      AgentOperation = "syncing-state-with-wfm"
	AgentOperationUpdatingStatusWithWfm    AgentOperation = "updating-app-status-with-wfm"
	AgentOperationSendingCapabilitiesToWfm AgentOperation = "sending-capabilities-to-wfm"
	AgentOperationDatabaseRead             AgentOperation = "database-read"
	AgentOperationDatabaseWrite            AgentOperation = "database-write"
	AgentOperationDatabaseDelete           AgentOperation = "database-delete"
	AgentOperationDatabaseUpdate           AgentOperation = "database-update"
)

// AgentError provides structured error handling
type AgentError struct {
	Component AgentComponent
	Operation AgentOperation
	Err       error
	Retryable bool
	Context   map[string]interface{}
}

func (e AgentError) Error() string {
	if len(e.Context) > 0 {
		return fmt.Sprintf("[%s:%s] %v (context: %v)", e.Component, e.Operation, e.Err, e.Context)
	}
	return fmt.Sprintf("[%s:%s] %v", e.Component, e.Operation, e.Err)
}

func NewAgentError(component AgentComponent, operation AgentOperation, err error, retryable bool) AgentError {
	return AgentError{
		Component: component,
		Operation: operation,
		Err:       err,
		Retryable: retryable,
	}
}

func (e AgentError) WithContext(key, value string) AgentError {
	e.Context[key] = value
	return e
}
