package models

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ApplicationDescription represents the root class for an application description
type ApplicationDescription struct {
	APIVersion         string               `json:"apiVersion" yaml:"apiVersion" validate:"required"`
	Kind               string               `json:"kind" yaml:"kind" validate:"required,eq=ApplicationDescription"`
	Metadata           Metadata             `json:"metadata" yaml:"metadata" validate:"required"`
	DeploymentProfiles []DeploymentProfile  `json:"deploymentProfiles" yaml:"deploymentProfiles" validate:"required,dive"`
	Parameters         map[string]Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty" validate:"dive"`
	Configuration      *Configuration       `json:"configuration,omitempty" yaml:"configuration,omitempty"`
}

// Metadata contains metadata about the application
type Metadata struct {
	ID          string  `json:"id" yaml:"id" validate:"required,pattern=^[a-z0-9-]{1,200}$"`
	Name        string  `json:"name" yaml:"name" validate:"required"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string  `json:"version" yaml:"version" validate:"required"`
	Catalog     Catalog `json:"catalog" yaml:"catalog" validate:"required"`
}

// Catalog contains catalog metadata for displaying the application
type Catalog struct {
	Application  *ApplicationMetadata `json:"application,omitempty" yaml:"application,omitempty"`
	Author       []Author             `json:"author,omitempty" yaml:"author,omitempty" validate:"dive"`
	Organization []Organization       `json:"organization" yaml:"organization" validate:"required,dive"`
}

// ApplicationMetadata contains metadata specific to the application
type ApplicationMetadata struct {
	DescriptionFile string   `json:"descriptionFile,omitempty" yaml:"descriptionFile,omitempty"`
	Icon            string   `json:"icon,omitempty" yaml:"icon,omitempty"`
	LicenseFile     string   `json:"licenseFile,omitempty" yaml:"licenseFile,omitempty"`
	ReleaseNotes    string   `json:"releaseNotes,omitempty" yaml:"releaseNotes,omitempty"`
	Site            string   `json:"site,omitempty" yaml:"site,omitempty"`
	Tagline         string   `json:"tagline,omitempty" yaml:"tagline,omitempty"`
	Tags            []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Author contains information about the application's author
type Author struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty" validate:"email"`
}

// Organization contains information about the providing organization
type Organization struct {
	Name string `json:"name" yaml:"name" validate:"required"`
	Site string `json:"site,omitempty" yaml:"site,omitempty"`
}

type Component struct {
	Name       string              `json:"name" yaml:"name" validate:"required"`
	Properties ComponentProperties `json:"properties" yaml:"properties" validate:"required"`
}

type DeploymentProfile struct {
	Type       string      `json:"type" yaml:"type" validate:"required,pattern=^(helm\.v3|compose)$"`
	Components []Component `json:"components" yaml:"components" validate:"required,dive"`
}

// // DeploymentProfile represents a deployment configuration interface
// type DeploymentProfile interface {
// 	GetType() string
// 	GetComponents() []Component
// 	Validate() error
// }

// BaseDeploymentProfile provides common fields for deployment profiles
// type BaseDeploymentProfile struct {
// 	Type       string      `json:"type" yaml:"type" validate:"required,pattern=^(helm\.v3|compose)$"`
// 	Components []Component `json:"components" yaml:"components" validate:"required,dive"`
// }

// func (b *BaseDeploymentProfile) GetType() string {
// 	return b.Type
// }

// func (b *BaseDeploymentProfile) GetComponents() []Component {
// 	return b.Components
// }

// func (b *BaseDeploymentProfile) Validate() error {
// 	typePattern := regexp.MustCompile(`^(helm\.v3|compose)$`)
// 	if !typePattern.MatchString(b.Type) {
// 		return fmt.Errorf("invalid deployment type: %s", b.Type)
// 	}
// 	return nil
// }

// // HelmDeploymentProfile represents a Helm v3 deployment profile
// type HelmDeploymentProfile struct {
// 	BaseDeploymentProfile
// 	Components []HelmComponent `json:"components" yaml:"components" validate:"required,dive"`
// }

// func (h *HelmDeploymentProfile) GetComponents() []Component {
// 	components := make([]Component, len(h.Components))
// 	for i, comp := range h.Components {
// 		components[i] = &comp
// 	}
// 	return components
// }

// // ComposeDeploymentProfile represents a Compose deployment profile
// type ComposeDeploymentProfile struct {
// 	BaseDeploymentProfile
// 	Components []ComposeComponent `json:"components" yaml:"components" validate:"required,dive"`
// }

// func (c *ComposeDeploymentProfile) GetComponents() []Component {
// 	components := make([]Component, len(c.Components))
// 	for i, comp := range c.Components {
// 		components[i] = &comp
// 	}
// 	return components
// }

// Component represents a component interface
// type Component interface {
// 	GetName() string
// 	GetProperties() ComponentProperties
// 	Validate() error
// }

// // BaseComponent provides common fields for components
// type BaseComponent struct {
// 	Name       string              `json:"name" yaml:"name" validate:"required"`
// 	Properties ComponentProperties `json:"properties" yaml:"properties" validate:"required"`
// }

// func (b *BaseComponent) GetName() string {
// 	return b.Name
// }

// func (b *BaseComponent) GetProperties() ComponentProperties {
// 	return b.Properties
// }

// func (b *BaseComponent) Validate() error {
// 	namePattern := regexp.MustCompile(`^[a-z0-9-]+$`)
// 	if !namePattern.MatchString(b.Name) {
// 		return fmt.Errorf("invalid component name: %s", b.Name)
// 	}
// 	return nil
// }

// // HelmComponent represents a Helm component
// type HelmComponent struct {
// 	BaseComponent
// }

// // ComposeComponent represents a Compose component
// type ComposeComponent struct {
// 	BaseComponent
// }

// ComponentProperties contains properties dictionary for component deployment details
type ComponentProperties struct {
	Repository      string `json:"repository,omitempty" yaml:"repository,omitempty"`
	Revision        string `json:"revision,omitempty" yaml:"revision,omitempty"`
	Wait            *bool  `json:"wait,omitempty" yaml:"wait,omitempty"`
	Timeout         string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	PackageLocation string `json:"packageLocation,omitempty" yaml:"packageLocation,omitempty"`
	KeyLocation     string `json:"keyLocation,omitempty" yaml:"keyLocation,omitempty"`
}

// Parameter defines a configurable parameter for the application
type Parameter struct {
	Name    string      `json:"name" yaml:"name" validate:"required"`
	Value   interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	Targets []Target    `json:"targets" yaml:"targets" validate:"required,dive"`
}

// Target specifies where the parameter applies in the deployment
type Target struct {
	Pointer    string   `json:"pointer" yaml:"pointer" validate:"required"`
	Components []string `json:"components" yaml:"components" validate:"required"`
}

// Configuration contains configuration layout and validation rules
type Configuration struct {
	Sections []Section `json:"sections" yaml:"sections" validate:"required,dive"`
	Schema   []Schema  `json:"schema" yaml:"schema" validate:"required,dive"`
}

// Section represents named sections within the configuration layout
type Section struct {
	Name     string    `json:"name" yaml:"name" validate:"required"`
	Settings []Setting `json:"settings" yaml:"settings" validate:"required,dive"`
}

// Setting represents individual configuration settings
type Setting struct {
	Parameter   string `json:"parameter" yaml:"parameter" validate:"required"`
	Name        string `json:"name" yaml:"name" validate:"required"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Immutable   *bool  `json:"immutable,omitempty" yaml:"immutable,omitempty"`
	Schema      string `json:"schema" yaml:"schema" validate:"required"`
}

type Schema struct {
	Name     string `json:"name" yaml:"name" validate:"required"`
	DataType string `json:"dataType" yaml:"dataType" validate:"required"`
}

// // Schema defines data type and rules for validating user provided parameter values
// type Schema interface {
// 	GetName() string
// 	GetDataType() string
// 	Validate(value interface{}) error
// }

// // BaseSchema provides common fields for all schema types
// type BaseSchema struct {
// 	Name     string `json:"name" yaml:"name" validate:"required"`
// 	DataType string `json:"dataType" yaml:"dataType" validate:"required"`
// }

// func (b *BaseSchema) GetName() string {
// 	return b.Name
// }

// func (b *BaseSchema) GetDataType() string {
// 	return b.DataType
// }

// // TextValidationSchema extends schema for string/text validation
// type TextValidationSchema struct {
// 	BaseSchema
// 	AllowEmpty *bool  `json:"allowEmpty,omitempty" yaml:"allowEmpty,omitempty"`
// 	MinLength  *int   `json:"minLength,omitempty" yaml:"minLength,omitempty"`
// 	MaxLength  *int   `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
// 	RegexMatch string `json:"regexMatch,omitempty" yaml:"regexMatch,omitempty"`
// }

// func (t *TextValidationSchema) Validate(value interface{}) error {
// 	str, ok := value.(string)
// 	if !ok {
// 		return fmt.Errorf("expected string, got %T", value)
// 	}

// 	if t.AllowEmpty != nil && !*t.AllowEmpty && str == "" {
// 		return fmt.Errorf("empty value not allowed")
// 	}

// 	if t.MinLength != nil && len(str) < *t.MinLength {
// 		return fmt.Errorf("string too short: minimum %d characters", *t.MinLength)
// 	}

// 	if t.MaxLength != nil && len(str) > *t.MaxLength {
// 		return fmt.Errorf("string too long: maximum %d characters", *t.MaxLength)
// 	}

// 	if t.RegexMatch != "" {
// 		matched, err := regexp.MatchString(t.RegexMatch, str)
// 		if err != nil {
// 			return fmt.Errorf("invalid regex pattern: %v", err)
// 		}
// 		if !matched {
// 			return fmt.Errorf("string does not match pattern: %s", t.RegexMatch)
// 		}
// 	}

// 	return nil
// }

// // BooleanValidationSchema extends schema for boolean validation
// type BooleanValidationSchema struct {
// 	BaseSchema
// 	AllowEmpty *bool `json:"allowEmpty,omitempty" yaml:"allowEmpty,omitempty"`
// }

// func (b *BooleanValidationSchema) Validate(value interface{}) error {
// 	_, ok := value.(bool)
// 	if !ok {
// 		return fmt.Errorf("expected boolean, got %T", value)
// 	}
// 	return nil
// }

// // NumericIntegerValidationSchema extends schema for integer validation
// type NumericIntegerValidationSchema struct {
// 	BaseSchema
// 	AllowEmpty *bool `json:"allowEmpty,omitempty" yaml:"allowEmpty,omitempty"`
// 	MinValue   *int  `json:"minValue,omitempty" yaml:"minValue,omitempty"`
// 	MaxValue   *int  `json:"maxValue,omitempty" yaml:"maxValue,omitempty"`
// }

// func (n *NumericIntegerValidationSchema) Validate(value interface{}) error {
// 	var intVal int
// 	switch v := value.(type) {
// 	case int:
// 		intVal = v
// 	case int64:
// 		intVal = int(v)
// 	case float64:
// 		intVal = int(v)
// 	default:
// 		return fmt.Errorf("expected integer, got %T", value)
// 	}

// 	if n.MinValue != nil && intVal < *n.MinValue {
// 		return fmt.Errorf("value too small: minimum %d", *n.MinValue)
// 	}

// 	if n.MaxValue != nil && intVal > *n.MaxValue {
// 		return fmt.Errorf("value too large: maximum %d", *n.MaxValue)
// 	}

// 	return nil
// }

// // NumericDoubleValidationSchema extends schema for double validation
// type NumericDoubleValidationSchema struct {
// 	BaseSchema
// 	AllowEmpty   *bool    `json:"allowEmpty,omitempty" yaml:"allowEmpty,omitempty"`
// 	MinValue     *float64 `json:"minValue,omitempty" yaml:"minValue,omitempty"`
// 	MaxValue     *float64 `json:"maxValue,omitempty" yaml:"maxValue,omitempty"`
// 	MinPrecision *int     `json:"minPrecision,omitempty" yaml:"minPrecision,omitempty"`
// 	MaxPrecision *int     `json:"maxPrecision,omitempty" yaml:"maxPrecision,omitempty"`
// }

// func (n *NumericDoubleValidationSchema) Validate(value interface{}) error {
// 	var floatVal float64
// 	switch v := value.(type) {
// 	case float64:
// 		floatVal = v
// 	case float32:
// 		floatVal = float64(v)
// 	case int:
// 		floatVal = float64(v)
// 	case int64:
// 		floatVal = float64(v)
// 	default:
// 		return fmt.Errorf("expected number, got %T", value)
// 	}

// 	if n.MinValue != nil && floatVal < *n.MinValue {
// 		return fmt.Errorf("value too small: minimum %f", *n.MinValue)
// 	}

// 	if n.MaxValue != nil && floatVal > *n.MaxValue {
// 		return fmt.Errorf("value too large: maximum %f", *n.MaxValue)
// 	}

// 	return nil
// }

// // SelectValidationSchema extends schema for select options validation
// type SelectValidationSchema struct {
// 	BaseSchema
// 	AllowEmpty  *bool    `json:"allowEmpty,omitempty" yaml:"allowEmpty,omitempty"`
// 	Multiselect *bool    `json:"multiselect,omitempty" yaml:"multiselect,omitempty"`
// 	Options     []string `json:"options" yaml:"options" validate:"required"`
// }

// func (s *SelectValidationSchema) Validate(value interface{}) error {
// 	if s.Multiselect != nil && *s.Multiselect {
// 		// Handle array of values
// 		arr, ok := value.([]interface{})
// 		if !ok {
// 			return fmt.Errorf("expected array for multiselect, got %T", value)
// 		}
// 		for _, item := range arr {
// 			if !s.isValidOption(item) {
// 				return fmt.Errorf("invalid option: %v", item)
// 			}
// 		}
// 	} else {
// 		// Handle single value
// 		if !s.isValidOption(value) {
// 			return fmt.Errorf("invalid option: %v", value)
// 		}
// 	}
// 	return nil
// }

// func (s *SelectValidationSchema) isValidOption(value interface{}) bool {
// 	strVal := fmt.Sprintf("%v", value)
// 	for _, option := range s.Options {
// 		if option == strVal {
// 			return true
// 		}
// 	}
// 	return false
// }

// // Helper functions for creating deployment profiles
// func NewHelmDeploymentProfile(components []HelmComponent) *HelmDeploymentProfile {
// 	return &HelmDeploymentProfile{
// 		BaseDeploymentProfile: BaseDeploymentProfile{
// 			Type: "helm.v3",
// 		},
// 		Components: components,
// 	}
// }

// func NewComposeDeploymentProfile(components []ComposeComponent) *ComposeDeploymentProfile {
// 	return &ComposeDeploymentProfile{
// 		BaseDeploymentProfile: BaseDeploymentProfile{
// 			Type: "compose",
// 		},
// 		Components: components,
// 	}
// }

// func NewApplicationDescription(apiVersion, kind string, metadata Metadata, deploymentProfiles []DeploymentProfile, parameters map[string]Parameter, configuration *Configuration) *ApplicationDescription {
// 	return &ApplicationDescription{
// 		APIVersion:         apiVersion,
// 		Kind:               kind,
// 		Metadata:           metadata,
// 		DeploymentProfiles: deploymentProfiles,
// 		Parameters:         parameters,
// 		Configuration:      configuration,
// 	}
// }

// func NewMetadata(id, name, description, version string, catalog Catalog) Metadata {
// 	return Metadata{
// 		ID:          id,
// 		Name:        name,
// 		Description: description,
// 		Version:     version,
// 		Catalog:     catalog,
// 	}
// }

// func NewCatalog(application *ApplicationMetadata, author []Author, organization []Organization) Catalog {
// 	return Catalog{
// 		Application:  application,
// 		Author:       author,
// 		Organization: organization,
// 	}
// }

// func NewApplicationMetadata(descriptionFile, icon, licenseFile, releaseNotes, site, tagline string, tags []string) *ApplicationMetadata {
// 	return &ApplicationMetadata{
// 		DescriptionFile: descriptionFile,
// 		Icon:            icon,
// 		LicenseFile:     licenseFile,
// 		ReleaseNotes:    releaseNotes,
// 		Site:            site,
// 		Tagline:         tagline,
// 		Tags:            tags,
// 	}
// }

// func NewAuthor(name, email string) Author {
// 	return Author{
// 		Name:  name,
// 		Email: email,
// 	}
// }

// func NewOrganization(name, site string) Organization {
// 	return Organization{
// 		Name: name,
// 		Site: site,
// 	}
// }

// func NewHelmComponent(name string, properties ComponentProperties) *HelmComponent {
// 	return &HelmComponent{
// 		BaseComponent: BaseComponent{
// 			Name:       name,
// 			Properties: properties,
// 		},
// 	}
// }

// func NewComposeComponent(name string, properties ComponentProperties) *ComposeComponent {
// 	return &ComposeComponent{
// 		BaseComponent: BaseComponent{
// 			Name:       name,
// 			Properties: properties,
// 		},
// 	}
// }

// func NewParameter(name string, value interface{}, targets []Target) Parameter {
// 	return Parameter{
// 		Name:    name,
// 		Value:   value,
// 		Targets: targets,
// 	}
// }

// func NewSection(name string, settings []Setting) Section {
// 	return Section{
// 		Name:     name,
// 		Settings: settings,
// 	}
// }

// func NewSetting(parameter string, name, description string, immutable *bool, schema string) Setting {
// 	return Setting{
// 		Parameter:   parameter,
// 		Name:        name,
// 		Description: description,
// 		Immutable:   immutable,
// 		Schema:      schema,
// 	}
// }

// func NewConfiguration(sections []Section, schema []Schema) *Configuration {
// 	return &Configuration{
// 		Sections: sections,
// 		Schema:   schema,
// 	}
// }

// func NewSchema(name, dataType string) Schema {
// 	return &BaseSchema{
// 		Name:     name,
// 		DataType: dataType,
// 	}
// }

// func NewTextValidationSchema(name string, allowEmpty *bool, minLength, maxLength *int, regexMatch string) *TextValidationSchema {
// 	return &TextValidationSchema{
// 		BaseSchema: BaseSchema{
// 			Name:     name,
// 			DataType: "Text",
// 		},
// 		AllowEmpty: allowEmpty,
// 		MinLength:  minLength,
// 		MaxLength:  maxLength,
// 		RegexMatch: regexMatch,
// 	}
// }

// func NewBooleanValidationSchema(name string, allowEmpty *bool) *BooleanValidationSchema {
// 	return &BooleanValidationSchema{
// 		BaseSchema: BaseSchema{
// 			Name:     name,
// 			DataType: "Boolean",
// 		},
// 		AllowEmpty: allowEmpty,
// 	}
// }

// func NewNumericIntegerValidationSchema(name string, allowEmpty *bool, minValue, maxValue *int) *NumericIntegerValidationSchema {
// 	return &NumericIntegerValidationSchema{
// 		BaseSchema: BaseSchema{
// 			Name:     name,
// 			DataType: "NumericInteger",
// 		},
// 		AllowEmpty: allowEmpty,
// 		MinValue:   minValue,
// 		MaxValue:   maxValue,
// 	}
// }

// func NewNumericDoubleValidationSchema(name string, allowEmpty *bool, minValue, maxValue *float64, minPrecision, maxPrecision *int) *NumericDoubleValidationSchema {
// 	return &NumericDoubleValidationSchema{
// 		BaseSchema: BaseSchema{
// 			Name:     name,
// 			DataType: "NumericDouble",
// 		},
// 		AllowEmpty:   allowEmpty,
// 		MinValue:     minValue,
// 		MaxValue:     maxValue,
// 		MinPrecision: minPrecision,
// 		MaxPrecision: maxPrecision,
// 	}
// }

// func NewSelectValidationSchema(name string, allowEmpty *bool, multiselect *bool, options []string) *SelectValidationSchema {
// 	return &SelectValidationSchema{
// 		BaseSchema: BaseSchema{
// 			Name:     name,
// 			DataType: "Select",
// 		},
// 		AllowEmpty:  allowEmpty,
// 		Multiselect: multiselect,
// 		Options:     options,
// 	}
// }

// func NewHelmDeploymentProfileWithComponents(components []HelmComponent) *HelmDeploymentProfile {
// 	return &HelmDeploymentProfile{
// 		BaseDeploymentProfile: BaseDeploymentProfile{
// 			Type: "helm.v3",
// 		},
// 		Components: components,
// 	}
// }

// func NewComposeDeploymentProfileWithComponents(components []ComposeComponent) *ComposeDeploymentProfile {
// 	return &ComposeDeploymentProfile{
// 		BaseDeploymentProfile: BaseDeploymentProfile{
// 			Type: "compose",
// 		},
// 		Components: components,
// 	}
// }

// func NewBaseSchema(name, dataType string) *BaseSchema {
// 	return &BaseSchema{
// 		Name:     name,
// 		DataType: dataType,
// 	}
// }

type ApplicationDescriptionFormat string

const (
	ApplicationDescriptionFormatYAML ApplicationDescriptionFormat = "yaml"
	ApplicationDescriptionFormatJSON ApplicationDescriptionFormat = "json"
)

func ParseApplicationDescription(r io.Reader, format ApplicationDescriptionFormat) (ApplicationDescription, error) {
	description := ApplicationDescription{}
	switch format {
	case ApplicationDescriptionFormatYAML:
		if err := yaml.NewDecoder(r).Decode(&description); err != nil {
			return description, err
		}
	case ApplicationDescriptionFormatJSON:
		if err := json.NewDecoder(r).Decode(&description); err != nil {
			return description, err
		}
	default:
		return description, fmt.Errorf("unknown format: %s", format)
	}
	return description, nil
}
