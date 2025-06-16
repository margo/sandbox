// margo.go
package margo

// api/pkg/apis/v1alpha1/providers/target/margo
import (
	"context"
	"fmt"

	"github.com/eclipse-symphony/symphony/api/pkg/apis/v1alpha1/model"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/providers"
	"github.com/kr/pretty"
)

type MargoProvider struct{}

func (p *MargoProvider) Init(config providers.IProviderConfig) error {
	fmt.Println("Margo provider initialized with config:", config)
	return nil
}

// get validation rules
func (p *MargoProvider) GetValidationRule(ctx context.Context) model.ValidationRule {
	fmt.Printf("Margo:GetValidationRule")
	return model.ValidationRule{}
}

// get current component states from a target. The desired state is passed in as a reference
func (p *MargoProvider) Get(ctx context.Context, deployment model.DeploymentSpec, references []model.ComponentStep) ([]model.ComponentSpec, error) {
	fmt.Print("Margo:Get", "deployment", pretty.Sprint(deployment))
	fmt.Print("Margo:Get", "references", pretty.Sprint(references))
	return nil, nil
}

// apply components to a target
func (p *MargoProvider) Apply(ctx context.Context, deployment model.DeploymentSpec, step model.DeploymentStep, isDryRun bool) (map[string]model.ComponentResultSpec, error) {
	fmt.Printf("Margo:Apply")
	return nil, nil
}
