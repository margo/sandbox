package main

import (
    "fmt"
    "regexp"
)

func ValidateAppDescription(
    app *ApplicationDescription,
    report *ValidationReport,
) error {

    validateTopLevel(
        app,
        report,
    )

    validateMetadata(
        app,
        report,
    )

    componentNames, _ :=
        validateDeploymentProfiles(
            app,
            report,
        )

    schemas, _ :=
        validateSchemas(
            app,
            report,
        )

    validateConfiguration(
        app,
        report,
        schemas,
    )

    validateParameterTargets(
        app,
        report,
        componentNames,
    )

    return nil
}


func validateTopLevel(
    app *ApplicationDescription,
    report *ValidationReport,
) error {

    check(
        report,
        "apiVersion",
        "string",
        "Required (non-empty)",
    )

    if app.APIVersion == "" {

        fail(
            report,
            "(missing)",
            "Application version information is missing.",
        )

    } else {

        pass(
            report,
            app.APIVersion,
            fmt.Sprintf(
                "Application version '%s' conforms to the required format.",
                app.APIVersion,
            ),
        )
    }

    check(
        report,
        "kind",
        "string",
        "ApplicationDescription",
    )

    if app.Kind == "" {

        fail(
            report,
            "(missing)",
            "Application type is missing.",
        )

    } else if app.Kind != "ApplicationDescription" {

        fail(
            report,
            app.Kind,
            fmt.Sprintf(
                "Application type must be 'ApplicationDescription'. Found '%s'.",
                app.Kind,
            ),
        )

    } else {

        pass(
            report,
            app.Kind,
            fmt.Sprintf(
                "Application type is valid. Found '%s'.",
                app.Kind,
            ),
        )
    }

    check(
        report,
        "id",
        "string",
        "lowercase letters, numbers and dashes only, max length=200",
    )

    re := regexp.MustCompile(
        `^[a-z0-9-]{1,200}$`,
    )

    if app.ID == "" {

        fail(
            report,
            "(missing)",
            "Application identifier is required.",
        )

    } else if !re.MatchString(app.ID) {

        fail(
            report,
            app.ID,
            fmt.Sprintf(
                "Application ID format is invalid. Value='%s'.",
                app.ID,
            ),
        )

    } else {

        pass(
            report,
            app.ID,
            fmt.Sprintf(
                "Application ID '%s' is valid.",
                app.ID,
            ),
        )
    }

    return nil
}

func validateDeploymentProfiles(
    app *ApplicationDescription,
    report *ValidationReport,
) (map[string]bool, error) {

    check(
        report,
        "deploymentProfile",
        "array",
        "At least one deployment profile",
    )

    if len(app.DeploymentProfile) == 0 {

        fail(
            report,
            "0 deployment profile(s)",
            "At least one deployment profile must be provided.",
        )

    } else {

        pass(
            report,
            fmt.Sprintf(
                "%d deployment profile(s)",
                len(app.DeploymentProfile),
            ),
            fmt.Sprintf(
                "%d deployment profile(s) found.",
                len(app.DeploymentProfile),
            ),
        )
    }

    componentNames := make(map[string]bool)

    for _, profile := range app.DeploymentProfile {

        check(
            report,
            "deploymentProfile.type",
            "string",
            "helm | compose",
        )

        if profile.Type == "" {

            fail(
                report,
                "(missing)",
                "Deployment profile type is missing.",
            )

        } else if profile.Type != "helm" &&
            profile.Type != "compose" {

            fail(
                report,
                profile.Type,
                fmt.Sprintf(
                    "Deployment type '%s' is not supported. Use 'helm' or 'compose'.",
                    profile.Type,
                ),
            )

        } else {

            pass(
                report,
                profile.Type,
                fmt.Sprintf(
                    "Deployment profile type '%s' is valid.",
                    profile.Type,
                ),
            )
        }

        check(
            report,
            "deploymentProfile.id",
            "string",
            "Required (non-empty)",
        )

        if profile.ID == "" {

            fail(
                report,
                "(missing)",
                "Deployment profile identifier is missing.",
            )

        } else {

            pass(
                report,
                profile.ID,
                fmt.Sprintf(
                    "Deployment profile '%s' is configured using '%s'.",
                    profile.ID,
                    profile.Type,
                ),
            )
        }

        if len(profile.Components) == 0 {

            fail(
                report,
                "0 component(s)",
                fmt.Sprintf(
                    "deploymentProfile '%s' has no components.",
                    profile.ID,
                ),
            )
        }

        for _, component := range profile.Components {

            check(
                report,
                "component.name",
                "string",
                "Required (non-empty)",
            )

            if component.Name == "" {

                fail(
                    report,
                    "(missing)",
                    "A component name is missing.",
                )

            } else {

                componentNames[component.Name] = true

                pass(
                    report,
                    component.Name,
                    fmt.Sprintf(
                        "Component '%s' is configured.",
                        component.Name,
                    ),
                )
            }

            if len(component.Properties) == 0 {

                fail(
                    report,
                    "0 properties",
                    fmt.Sprintf(
                        "Configuration details are missing for component '%s'.",
                        component.Name,
                    ),
                )
            }

            if profile.Type == "helm" {

                check(
                    report,
                    "repository",
                    "string",
                    "Repository URL required",
                )

                repository, ok :=
                    component.Properties["repository"]

                if !ok {

                    fail(
                        report,
                        "(missing)",
                        fmt.Sprintf(
                            "Repository information is missing for component '%s'.",
                            component.Name,
                        ),
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", repository),
                        fmt.Sprintf(
                            "Repository information is available for component '%s'.",
                            component.Name,
                        ),
                    )
                }

                check(
                    report,
                    "revision",
                    "string",
                    "Required (non-empty)",
                )

                revision, ok :=
                    component.Properties["revision"]

                if !ok {

                    fail(
                        report,
                        "(missing)",
                        fmt.Sprintf(
                            "Revision information is missing for component '%s'.",
                            component.Name,
                        ),
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", revision),
                        fmt.Sprintf(
                            "Revision information is available for component '%s'.",
                            component.Name,
                        ),
                    )
                }
            }

            if profile.Type == "compose" {

                check(
                    report,
                    "compose.packageLocation",
                    "string",
                    "Required package location",
                )

                packageLocation, ok :=
                    component.Properties["packageLocation"]

                if !ok {

                    fail(
                        report,
                        "(missing)",
                        fmt.Sprintf(
                            "Package location is missing for component '%s'.",
                            component.Name,
                        ),
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", packageLocation),
                        fmt.Sprintf(
                            "Package location is configured for component '%s'.",
                            component.Name,
                        ),
                    )
                }
            }
        }
    }

    return componentNames, nil
}


func validateSchemas(
    app *ApplicationDescription,
    report *ValidationReport,
) (map[string]bool, error) {

    schemas := make(map[string]bool)

    for _, schema := range app.Configuration.Schema {

        check(
            report,
            "schema",
            "string",
            "name and dataType required",
        )

        if schema.Name == "" {

            fail(
                report,
                "(missing)",
                "Schema name is missing.",
            )

        } else if schema.DataType == "" {

            fail(
                report,
                "(missing)",
                fmt.Sprintf(
                    "Data type is missing for schema '%s'.",
                    schema.Name,
                ),
            )

        } else {

            schemas[schema.Name] = true

            pass(
                report,
                schema.Name,
                fmt.Sprintf(
                    "Schema '%s' is configured correctly. DataType='%s', AllowEmpty=%t.",
                    schema.Name,
                    schema.DataType,
                    schema.AllowEmpty,
                ),
            )
        }
    }

    return schemas, nil
}


func validateConfiguration(
    app *ApplicationDescription,
    report *ValidationReport,
    schemas map[string]bool,
) error {

    for _, section := range app.Configuration.Sections {

        check(
            report,
            "configuration.section",
            "string",
            "Required (non-empty)",
        )

        pass(
            report,
            section.Name,
            fmt.Sprintf(
                "Configuration section '%s' has been identified.",
                section.Name,
            ),
        )

        for _, setting := range section.Settings {

            check(
                report,
                fmt.Sprintf(
                    "setting.parameter.%s",
                    setting.Parameter,
                ),
                "reference",
                "Must match a parameter definition",
            )

            if _, ok :=
                app.Parameters[setting.Parameter]; !ok {

                fail(
                    report,
                    setting.Parameter,
                    fmt.Sprintf(
                        "Configuration uses parameter '%s', but that parameter is not defined.",
                        setting.Parameter,
                    ),
                )

            } else {

                pass(
                    report,
                    setting.Parameter,
                    fmt.Sprintf(
                        "Parameter '%s' is available.",
                        setting.Parameter,
                    ),
                )
            }

            if setting.Schema != "" {

                check(
                    report,
                    fmt.Sprintf(
                        "setting.parameter.%s.schema",
                        setting.Parameter,
                    ),
                    "reference",
                    "Must match a schema definition",
                )

                if !schemas[setting.Schema] {

                    fail(
                        report,
                        setting.Schema,
                        fmt.Sprintf(
                            "Schema '%s' is referenced but not defined.",
                            setting.Schema,
                        ),
                    )

                } else {

                    pass(
                        report,
                        setting.Schema,
                        fmt.Sprintf(
                            "Schema '%s' is available.",
                            setting.Schema,
                        ),
                    )
                }
            }
        }
    }

    return nil
}

func validateParameterTargets(
    app *ApplicationDescription,
    report *ValidationReport,
    componentNames map[string]bool,
) error {

    for parameterName, parameter :=
        range app.Parameters {

        for _, target :=
            range parameter.Targets {

            for _, component :=
                range target.Components {

                check(
                    report,
                    fmt.Sprintf(
                        "parameter.%s",
                        parameterName,
                    ),
                    "reference",
                    "Must match a deployment component",
                )

                if !componentNames[component] {

                    fail(
                        report,
                        component,
                        fmt.Sprintf(
                            "Parameter '%s' references component '%s', but that component is not defined.",
                            parameterName,
                            component,
                        ),
                    )

                } else {

                    pass(
                        report,
                        component,
                        fmt.Sprintf(
                            "Parameter '%s' is successfully mapped to component '%s'. Pointer=%s",
                            parameterName,
                            component,
                            target.Pointer,
                        ),
                    )
                }
            }
        }
    }

    return nil
}

func validateMetadata(
    app *ApplicationDescription,
    report *ValidationReport,
) error {

    check(
        report,
        "metadata.name",
        "string",
        "Required (non-empty)",
    )

    if app.Metadata.Name == "" {

        fail(
            report,
            "(missing)",
            "Application name is missing.",
        )

    } else {

        pass(
            report,
            app.Metadata.Name,
            fmt.Sprintf(
                "Application name '%s' conforms to the required format.",
                app.Metadata.Name,
            ),
        )
    }

    check(
        report,
        "metadata.version",
        "string",
        "Required (non-empty)",
    )

    if app.Metadata.Version == "" {

        fail(
            report,
            "(missing)",
            "Application version is missing.",
        )

    } else {

        pass(
            report,
            app.Metadata.Version,
            fmt.Sprintf(
                "Application version '%s' conforms to the required format.",
                app.Metadata.Version,
            ),
        )
    }

    check(
        report,
        "metadata.catalog.organization",
        "array",
        "At least one organization",
    )

    if len(app.Metadata.Catalog.Organization) == 0 {

        fail(
            report,
            "0 organization(s)",
            "At least one organization must be specified in the catalog.",
        )

    } else {

        pass(
            report,
            fmt.Sprintf(
                "%d organization(s)",
                len(app.Metadata.Catalog.Organization),
            ),
            fmt.Sprintf(
                "%d organization(s) found in catalog.",
                len(app.Metadata.Catalog.Organization),
            ),
        )
    }

    for _, org := range app.Metadata.Catalog.Organization {

        check(
            report,
            "metadata.catalog.organization.name",
            "string",
            "Required (non-empty)",
        )

        if org.Name == "" {

            fail(
                report,
                "(missing)",
                "An organization name must be provided.",
            )

        } else {

            pass(
                report,
                org.Name,
                fmt.Sprintf(
                    "Organization '%s' is registered in the catalog.",
                    org.Name,
                ),
            )
        }
    }

    return nil
}

// func check(report *ValidationReport, msg string) {
//     fmt.Println("Validate", msg)
//     report.Check(msg)
// }

// func pass(report *ValidationReport, msg string) {
//     fmt.Println("PASS", msg)
//     report.Pass(msg)
// }

// func fail(report *ValidationReport, msg string) error {
//     fmt.Println("FAIL", msg)
//     report.Fail(msg)
//     return fmt.Errorf(msg)
// }

func check(
    report *ValidationReport,
    field string,
    dataType string,
    expected string,
) {

    fmt.Println("Validate", field)

    report.Check(
        field,
        dataType,
        expected,
    )
}

func pass(
    report *ValidationReport,
    actual string,
    details string,
) {

    fmt.Println("PASS", details)

    report.Pass(
        actual,
        details,
    )
}

func fail(
    report *ValidationReport,
    actual string,
    details string,
) {

    fmt.Println("FAIL", details)

    report.Fail(
        actual,
        details,
    )
}
