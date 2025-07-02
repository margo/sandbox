package models

type ApplicationPackageOp string

const (
	ApplicationPackageOpOnboard ApplicationPackageOp = "ONBOARD"
	ApplicationPackageOpDeboard ApplicationPackageOp = "DEBOARD"
	ApplicationPackageOpStage   ApplicationPackageOp = "STAGE"
	ApplicationPackageOpUnstage ApplicationPackageOp = "UNSTAGE"
)

type ApplicationPackageOperationState struct {
	Op        ApplicationPackageOp
	OpStatus  OpStatus
	OpDetails interface{}
}
