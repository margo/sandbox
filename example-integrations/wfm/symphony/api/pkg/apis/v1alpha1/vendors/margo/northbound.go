package margo

import (
	"github.com/eclipse-symphony/symphony/api/pkg/apis/v1alpha1/managers/solutions"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/managers"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/observability"
	observ_utils "github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/observability/utils"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/providers"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/providers/pubsub"
	"github.com/eclipse-symphony/symphony/coa/pkg/apis/v1alpha2/vendors"
	"github.com/eclipse-symphony/symphony/coa/pkg/logger"
	"github.com/valyala/fasthttp"
)

var uLog = logger.NewLogger("coa.runtime")

type MargoSouthboundVendor struct {
	vendors.Vendor
	MargoManager     *solutions.MargoManager
	SolutionsManager *solutions.SolutionsManager
}

func (o *MargoSouthboundVendor) GetInfo() vendors.VendorInfo {
	return vendors.VendorInfo{
		Version:  o.Vendor.Version,
		Name:     "MargoSouthbound",
		Producer: "Margo",
	}
}

func (e *MargoSouthboundVendor) Init(config vendors.VendorConfig, factories []managers.IManagerFactroy, providers map[string]map[string]providers.IProvider, pubsubProvider pubsub.IPubSubProvider) error {
	err := e.Vendor.Init(config, factories, providers, pubsubProvider)
	if err != nil {
		return err
	}
	for _, m := range e.Managers {
		if c, ok := m.(*margo.MargoSouthboundVendor); ok {
			e.MargoManager = c
		}
	}
	if e.MargoManager == nil {
		return v1alpha2.NewCOAError(nil, "margo manager is not supplied", v1alpha2.MissingConfig)
	}
	return nil
}

func (o *MargoSouthboundVendor) GetEndpoints() []v1alpha2.Endpoint {
	route := "margo/v1"
	if o.Route != "" {
		route = o.Route
	}
	return []v1alpha2.Endpoint{
		{
			Methods: []string{fasthttp.MethodPost},
			Route:   route + "/applications",
			Version: o.Version,
			Handler: o.onboardApplication,
		},
		{
			Methods:    []string{fasthttp.MethodGet},
			Route:      route + "/applications",
			Version:    o.Version,
			Handler:    o.listApplications,
			Parameters: []string{"id?", "name?", "type?"},
		},
		{
			Methods:    []string{fasthttp.MethodGet},
			Route:      route + "/applications",
			Version:    o.Version,
			Handler:    o.getApplication,
			Parameters: []string{"id?"},
		},
		{
			Methods:    []string{fasthttp.MethodDelete},
			Route:      route + "/applications",
			Version:    o.Version,
			Handler:    o.deleteApplication,
			Parameters: []string{"id?"},
		},
	}
}

func (c *MargoSouthboundVendor) onboardApplication(request v1alpha2.COARequest) v1alpha2.COAResponse {
	pCtx, span := observability.StartSpan("Margo Southbound Vendor", request.Context, &map[string]string{
		"method": "onboardApplication",
		"route":  request.Route,
		"verb":   request.Method,
	})
	defer span.End()
	uLog.InfofCtx(pCtx, "V (MargoSouthboundVendor): onboardApplication, method: %s", request.Method)

	margoSpec, err := margoModels.ParseApplication(request.Body)
	coaErr := v1alpha2.NewCOAError(err, "Failed to parse the request", v1alpha2.BadRequest)
	if err != nil {
		return v1alpha2.COAResponse{
			State: v1alpha2.GetErrorState(coaErr),
			Body:  []byte(coaErr.Error()),
		}
	}

	err = c.MargoManager.OnboardApplication(margoSpec)
	if err != nil {
		return v1alpha2.COAResponse{
			State: v1alpha2.GetErrorState(coaErr),
			Body:  []byte(coaErr.Error()),
		}
	}

	symphonySolution, coaErr := c.MargoManager.ConvertToSolutionSpec(id, margoSpec)
	if err != nil {
		return v1alpha2.COAResponse{
			State: v1alpha2.GetErrorState(coaErr),
			Body:  []byte(coaErr.Error()),
		}
	}

	err := c.SolutionsManager.UpsertState(ctx, id, solution)
	if err != nil {
		uLog.ErrorfCtx(ctx, "V (Solutions): onboardApplication failed - %s", err.Error())
		return observ_utils.CloseSpanWithCOAResponse(span, v1alpha2.COAResponse{
			State: v1alpha2.GetErrorState(err),
			Body:  []byte(err.Error()),
		})
	}

	// c.Vendor.Context.Publish("trail", v1alpha2.Event{
	// 	Body: []v1alpha2.Trail{
	// 		{
	// 			Origin:  c.Vendor.Context.SiteInfo.SiteId,
	// 			Catalog: strCat,
	// 			Type:    "solutions.solution.symphony/v1",
	// 			Properties: map[string]interface{}{
	// 				"spec": solution,
	// 			},
	// 		},
	// 	},
	// 	Metadata: map[string]string{
	// 		"namespace": namespace,
	// 	},
	// 	Context: ctx,
	// })
	return observ_utils.CloseSpanWithCOAResponse(span, v1alpha2.COAResponse{
		State: v1alpha2.OK,
	})

	uLog.ErrorCtx(pCtx, "V (Solutions): onboardApplication failed - 405 method not allowed")
	resp := v1alpha2.COAResponse{
		State:       v1alpha2.MethodNotAllowed,
		Body:        []byte("{\"result\":\"405 - method not allowed\"}"),
		ContentType: "application/json",
	}
	observ_utils.UpdateSpanStatusFromCOAResponse(span, resp)
	return resp
}

func (c *MargoSouthboundVendor) listApplications(request v1alpha2.COARequest) v1alpha2.COAResponse {
	resp := v1alpha2.COAResponse{
		State:       v1alpha2.MethodNotAllowed,
		Body:        []byte("{\"result\":\"405 - method not allowed\"}"),
		ContentType: "application/json",
	}
	return resp
}

func (c *MargoSouthboundVendor) getApplication(request v1alpha2.COARequest) v1alpha2.COAResponse {
	resp := v1alpha2.COAResponse{
		State:       v1alpha2.MethodNotAllowed,
		Body:        []byte("{\"result\":\"405 - method not allowed\"}"),
		ContentType: "application/json",
	}
	return resp
}

func (c *MargoSouthboundVendor) deleteApplication(request v1alpha2.COARequest) v1alpha2.COAResponse {
	resp := v1alpha2.COAResponse{
		State:       v1alpha2.MethodNotAllowed,
		Body:        []byte("{\"result\":\"405 - method not allowed\"}"),
		ContentType: "application/json",
	}
	return resp
}
