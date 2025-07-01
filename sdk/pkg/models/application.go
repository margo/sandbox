package models

import (
	"encoding/json"

	"github.com/margo/dev-repo/sdk/utils"
)

type Application struct {
	AppId string
	ApplicationState[string]
	ApplicationDescription
}

func NewApplication(desc ApplicationDescription, op ApplicationOp, status OpStatus) Application {
	return Application{
		AppId: utils.GenerateAppId(),
		ApplicationState: ApplicationState[string]{
			CurrentOperation: ApplicationOpOnboardingRequested,
			OpStatus:         OpStatusPending,
			OpDetails:        "",
		},
	}
}

func ParseApplicationFromBytes(data []byte) (Application, error) {
	app := Application{}
	if err := json.Unmarshal(data, &app); err != nil {
		return app, err
	}
	return Application{}, nil
}
