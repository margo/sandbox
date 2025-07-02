package models

import (
	"encoding/json"

	"github.com/margo/dev-repo/sdk/utils"
)

type Application struct {
	AppId string
	ApplicationOperationState
	ApplicationDescription
}

func NewApplication(desc ApplicationDescription, op ApplicationOp, status OpStatus, details string) Application {
	return Application{
		AppId: utils.GenerateAppId(),
		ApplicationOperationState: ApplicationOperationState{
			Op:        op,
			OpStatus:  status,
			OpDetails: details,
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
