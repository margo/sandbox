package models

type OpStatus string

const (
	OpStatusSuccess OpStatus = "SUCCESS"
	OpStatusFailed  OpStatus = "FAILED"
	OpStatusPending OpStatus = "PENDING"
	OpStatusUnknown OpStatus = "UNKNOWN"
)
