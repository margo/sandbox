package models

type AppPkgOpStatus string

const (
	AppPkgOpStatusSuccess AppPkgOpStatus = "SUCCESS"
	AppPkgOpStatusFailed  AppPkgOpStatus = "FAILED"
	AppPkgOpStatusPending AppPkgOpStatus = "PENDING"
	AppPkgOpStatusUnknown AppPkgOpStatus = "UNKNOWN"
)
