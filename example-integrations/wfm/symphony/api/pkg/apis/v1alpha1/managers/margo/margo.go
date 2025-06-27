package margo

import (
	"context"

	"github.com/eclipse-symphony/symphony/api/pkg/apis/v1alpha1/model"
	"github.com/eclipse-symphony/symphony/api/pkg/apis/v1alpha1/validation"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/contexts"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/managers"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/providers"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/providers/states"
	"github.com/eclipse-symphony/symphony/coa/pkg/logger"
	margoModels "github.com/margo/dev-repo/sdk/pkg/models"
)

var margoLog = logger.NewLogger("coa.runtime")

type MargoManager struct {
	managers.Manager
	StateProvider  states.IStateProvider
	needValidate   bool
	MargoValidator validation.MargoValidator
}

func (s *MargoManager) Init(context *contexts.VendorContext, config managers.ManagerConfig, providers map[string]providers.IProvider) error {
	err := s.Manager.Init(context, config, providers)
	if err != nil {
		return err
	}
	stateprovider, err := managers.GetPersistentStateProvider(config, providers)
	if err == nil {
		s.StateProvider = stateprovider
	} else {
		return err
	}
	s.needValidate = managers.NeedObjectValidate(config, providers)
	if s.needValidate {
		// Turn off validation of differnt types: https://github.com/eclipse-symphony/symphony/issues/445
		s.MargoValidator = validation.NewMargoValidator()
	}
	return nil
}

func (s *MargoManager) OnboardApplication(context context.Context, spec margoModels.ApplicationDescription) (string, error) {
	// generate unique identifier for the onboarded application
	return "", nil
}

func (s *MargoManager) ListApplications(context context.Context) error {
	return nil
}

func (s *MargoManager) GetApplication(context context.Context) error {
	return nil
}

func (s *MargoManager) DeleteApplication(context context.Context) error {
	return nil
}

// TODO: move this to some suitable package
func (s *MargoManager) ConvertToSolution(context context.Context, appId string, spec margoModels.ApplicationDescription) (model.SolutionState, error) {
	return model.SolutionState{}, nil
}

func (s *MargoManager) reconcil(context context.Context) error {
	return nil
}

func (s *MargoManager) WatchAppChanges(context context.Context) error {
	return nil
}

// GetCampaign retrieves a CampaignSpec object by name
func (s *MargoManager) Shutdown(ctx context.Context) error {
	return nil
}
