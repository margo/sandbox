package main

type ApplicationDescription struct {
    APIVersion        string                         `yaml:"apiVersion"`
    Kind              string                         `yaml:"kind"`
    ID                string                         `yaml:"id"`
    Metadata          Metadata                       `yaml:"metadata"`
    DeploymentProfile []DeploymentProfile            `yaml:"deploymentProfiles"`
    Parameters        map[string]Parameter           `yaml:"parameters"`
    Configuration     Configuration                  `yaml:"configuration"`
}

type Metadata struct {
    Name        string  `yaml:"name"`
    Version     string  `yaml:"version"`
    Catalog     Catalog `yaml:"catalog"`
}

type Catalog struct {
    Organization []Organization `yaml:"organization"`
}

type Organization struct {
    Name string `yaml:"name"`
}

type DeploymentProfile struct {
    Type       string      `yaml:"type"`
    ID         string      `yaml:"id"`
    Components []Component `yaml:"components"`
}

type Component struct {
    Name       string                 `yaml:"name"`
    Properties map[string]interface{} `yaml:"properties"`
}

type Parameter struct {
    Value   interface{} `yaml:"value"`
    Targets []Target    `yaml:"targets"`
}

type Target struct {
    Pointer    string   `yaml:"pointer"`
    Components []string `yaml:"components"`
}

type Configuration struct {
    Sections []Section       `yaml:"sections"`
    Schema   []SchemaDef     `yaml:"schema"`
}

type Section struct {
    Name     string    `yaml:"name"`
    Settings []Setting `yaml:"settings"`
}

type Setting struct {
    Parameter   string `yaml:"parameter"`
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Schema      string `yaml:"schema"`
}

type SchemaDef struct {
    Name       string `yaml:"name"`
    DataType   string `yaml:"dataType"`
    AllowEmpty bool   `yaml:"allowEmpty"`
}