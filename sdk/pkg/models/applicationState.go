package models

type ApplicationOp string

const (
	ApplicationOpOnboardingRequested ApplicationOp = "OnboardingRequested"
	ApplicationOpOnboarded           ApplicationOp = "Onboarded"
	ApplicationOpActive              ApplicationOp = "Active"
	ApplicationOpInactive            ApplicationOp = "Inactive"
	ApplicationOpDeletionRequested   ApplicationOp = "DeletionRequested"
	ApplicationOpDeleted             ApplicationOp = "Deleted"
)

type OpDetailsConstraint interface {
	// interface{}
	string | interface{}
}

type ApplicationState[OpConstraint OpDetailsConstraint] struct {
	CurrentOperation ApplicationOp
	OpStatus         OpStatus
	OpDetails        OpConstraint
}
