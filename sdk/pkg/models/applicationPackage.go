package models

type ApplicationPackage struct {
	Id          string
	OpState     ApplicationPackageOperationState
	Description *ApplicationDescription // mandatory field
	Resources   map[string][]byte       // omitempty, *ApplicationResources  // optional field //map[string][]byte // filename -> content
}

type ApplicationResources struct {
	// icon, releasenotes, license file..
}
