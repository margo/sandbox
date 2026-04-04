package sbi

import (
	"encoding/json"
	"time"

	"github.com/oapi-codegen/runtime"
)

// Legacy deployment profile compatibility for device-agent code that still
// consumes per-application deployment manifests.

const (
	Compose AppDeploymentProfileType = "compose"
	HelmV3  AppDeploymentProfileType = "helm.v3"
)

type AppDeploymentManifest struct {
	ApiVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Annotations       *map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
		CreationTimestamp *time.Time         `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`
		Id                *string            `json:"id,omitempty" yaml:"id,omitempty"`
		Labels            *map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
		Name              string             `json:"name" yaml:"name"`
		Namespace         *string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	} `json:"metadata" yaml:"metadata"`
	Spec AppDeploymentSpec `json:"spec" yaml:"spec"`
}

type AppDeploymentSpec struct {
	AppPackageRef struct {
		Id string `json:"id" yaml:"id"`
	} `json:"appPackageRef" yaml:"appPackageRef"`
	DeploymentProfile AppDeploymentProfile `json:"deploymentProfile" yaml:"deploymentProfile"`
	DeviceRef         interface{}          `json:"deviceRef,omitempty" yaml:"deviceRef,omitempty"`
	Parameters        *AppDeploymentParams `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type AppDeploymentProfile struct {
	Components []AppDeploymentProfile_Components_Item `json:"components" yaml:"components"`
	Type       AppDeploymentProfileType               `json:"type" yaml:"type"`
}

type AppDeploymentProfileType string

type AppDeploymentProfile_Components_Item struct {
	union json.RawMessage
}

type AppDeploymentParams map[string]AppParameterValue

type AppParameterValue struct {
	Targets []AppParameterTarget `json:"targets" yaml:"targets"`
	Value   interface{}          `json:"value" yaml:"value"`
}

type AppParameterTarget struct {
	Components []string `json:"components" yaml:"components"`
	Pointer    string   `json:"pointer" yaml:"pointer"`
}

type HelmApplicationDeploymentProfileComponent struct {
	Name       string `json:"name" yaml:"name"`
	Properties struct {
		Repository string  `json:"repository" yaml:"repository"`
		Revision   *string `json:"revision" yaml:"revision"`
		Timeout    *string `json:"timeout" yaml:"timeout"`
		Wait       *bool   `json:"wait" yaml:"wait"`
	} `json:"properties" yaml:"properties"`
}

type ComposeApplicationDeploymentProfileComponent struct {
	Name       string `json:"name" yaml:"name"`
	Properties struct {
		KeyLocation     *string `json:"keyLocation" yaml:"keyLocation"`
		PackageLocation string  `json:"packageLocation" yaml:"packageLocation"`
		Timeout         *string `json:"timeout" yaml:"timeout"`
		Wait            *bool   `json:"wait" yaml:"wait"`
	} `json:"properties" yaml:"properties"`
}

func (t AppDeploymentProfile_Components_Item) AsHelmApplicationDeploymentProfileComponent() (HelmApplicationDeploymentProfileComponent, error) {
	var body HelmApplicationDeploymentProfileComponent
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *AppDeploymentProfile_Components_Item) FromHelmApplicationDeploymentProfileComponent(v HelmApplicationDeploymentProfileComponent) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t *AppDeploymentProfile_Components_Item) MergeHelmApplicationDeploymentProfileComponent(v HelmApplicationDeploymentProfileComponent) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JsonMerge(t.union, b)
	t.union = merged
	return err
}

func (t AppDeploymentProfile_Components_Item) AsComposeApplicationDeploymentProfileComponent() (ComposeApplicationDeploymentProfileComponent, error) {
	var body ComposeApplicationDeploymentProfileComponent
	err := json.Unmarshal(t.union, &body)
	return body, err
}

func (t *AppDeploymentProfile_Components_Item) FromComposeApplicationDeploymentProfileComponent(v ComposeApplicationDeploymentProfileComponent) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

func (t *AppDeploymentProfile_Components_Item) MergeComposeApplicationDeploymentProfileComponent(v ComposeApplicationDeploymentProfileComponent) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JsonMerge(t.union, b)
	t.union = merged
	return err
}

func (t AppDeploymentProfile_Components_Item) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *AppDeploymentProfile_Components_Item) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}
