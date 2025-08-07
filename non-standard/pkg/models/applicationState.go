package models

type AppPkgOp string

const (
	AppPkgOpOnboard AppPkgOp = "ONBOARD"
	AppPkgOpDeboard AppPkgOp = "DEBOARD"
	AppPkgOpStage   AppPkgOp = "STAGE"
	AppPkgOpUnstage AppPkgOp = "UNSTAGE"
)
