package applicationCatalog

type ApplicationCatalog struct{}

func NewApplicationCatalog() *ApplicationCatalog {
	return &ApplicationCatalog{}
}

func (catalog *ApplicationCatalog) AddAppPackage() {}

func (catalog *ApplicationCatalog) RemoveAppPackage() {}

func (catalog *ApplicationCatalog) FindAppPacakge() {}

func (catalog *ApplicationCatalog) ListAppPackages() {}
