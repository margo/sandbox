package models

type ApplicationPackage struct {
	ApplicationDescription
	ApplicationResources
	Description *ApplicationDescription
	Resources   map[string][]byte // filename -> content
	RootPath    string
}

type ApplicationResources struct {
	// icon, releasenotes, license file..
}

func ParseApplicationPackageFromDir(dirAbsolutePath string) (ApplicationPackage, error) {
	return ApplicationPackage{}, nil
}

func (pack *ApplicationPackage) Validate() error {
	return nil
}
