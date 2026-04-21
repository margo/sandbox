package models

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/margo/sandbox/non-standard/generatedCode/wfm/nbi"
	"gopkg.in/yaml.v3"
)

type ApplicationDescriptionFormat string

const (
	ApplicationDescriptionFormatYAML ApplicationDescriptionFormat = "yaml"
	ApplicationDescriptionFormatJSON ApplicationDescriptionFormat = "json"
)

func ParseApplicationDescription(
	r io.Reader,
	format ApplicationDescriptionFormat,
) (nbi.AppDescription, error) {
	description := nbi.AppDescription{}
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
