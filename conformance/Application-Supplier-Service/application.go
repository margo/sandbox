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
            "API version is required but was not provided.",
        )

    } else {

        pass(
            report,
            app.APIVersion,
            "API version conforms to the required specification.",
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
            "Application type is required but was not provided.",
        )

    } else if app.Kind != "ApplicationDescription" {

        fail(
            report,
            app.Kind,
            "Application type does not conform to the required baseline. Expected 'ApplicationDescription'.",
        )

    } else {

        pass(
            report,
            app.Kind,
            "Application type conforms to the required baseline.",
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
            "Application identifier is required but was not provided.",
        )

    } else if !re.MatchString(app.ID) {

        fail(
            report,
            app.ID,
            "Application identifier does not conform to the required naming convention.",
        )

    } else {

        pass(
            report,
            app.ID,
            "Application identifier conforms to the required naming convention.",
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
            "At least one deployment profile is required.",
        )

    } else {

        pass(
            report,
            fmt.Sprintf(
                "%d deployment profile(s)",
                len(app.DeploymentProfile),
            ),
            "Deployment profile configuration conforms to the required baseline.",
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
                "Deployment profile type is required but was not provided.",
            )

        } else if profile.Type != "helm" &&
            profile.Type != "compose" {

            fail(
                report,
                profile.Type,
                "Deployment profile type does not conform to the supported deployment specifications.",
            )

        } else {

            pass(
                report,
                profile.Type,
                "Deployment profile type conforms to the supported deployment specifications.",
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
                "Deployment profile identifier is required but was not provided.",
            )

        } else {

            pass(
                report,
                profile.ID,
                "Deployment profile configuration conforms to the defined specification.",
            )
        }

        if len(profile.Components) == 0 {

            fail(
                report,
                "0 component(s)",
                "Deployment profile does not contain any component definitions.",
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
                    "Component name is required but was not provided.",
                )

            } else {

                componentNames[component.Name] = true

                pass(
                    report,
                    component.Name,
                    "Component configuration conforms to the defined specification.",
                )
            }

            if len(component.Properties) == 0 {

                fail(
                    report,
                    "0 properties",
                    "Component configuration properties are not defined.",
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
                        "Repository configuration is required but was not provided.",
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", repository),
                        "Repository configuration conforms to the deployment requirements.",
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
                        "Revision information is required but was not provided.",
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", revision),
                        "Revision information conforms to the deployment requirements.",
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
                        "Package location is required but was not provided.",
                    )

                } else {

                    pass(
                        report,
                        fmt.Sprintf("%v", packageLocation),
                        "Package location conforms to the deployment requirements.",
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
                "Schema name is required but was not provided.",
            )

        } else if schema.DataType == "" {

            fail(
                report,
                "(missing)",
                fmt.Sprintf(
                    "Schema '%s' does not define a data type.",
                    schema.Name,
                ),
            )

        } else {

            schemas[schema.Name] = true

            pass(
                report,
                schema.Name,
                fmt.Sprintf(
                    "Schema '%s' conforms to the defined specification. Data type='%s', AllowEmpty=%t.",
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
            "Configuration section has been successfully identified.",
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
                        "Referenced parameter '%s' is not defined.",
                        setting.Parameter,
                    ),
                )

            } else {

                pass(
                    report,
                    setting.Parameter,
                    fmt.Sprintf(
                        "Parameter '%s' conforms to the defined specification.",
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
                            "Referenced schema '%s' is not defined.",
                            setting.Schema,
                        ),
                    )

                } else {

                    pass(
                        report,
                        setting.Schema,
                        fmt.Sprintf(
                            "Schema '%s' conforms to the defined specification.",
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
                                "Parameter '%s' references a component that does not conform to the defined deployment configuration.",
                                parameterName,
                            ),
                        )

                    } else {

                        pass(
                            report,
                            component,
                            fmt.Sprintf(
                                "Parameter '%s' conforms to the defined component mapping requirements. Pointer='%s'.",
                                parameterName,
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
            "Application name is required but was not provided.",
        )

    } else {

        pass(
            report,
            app.Metadata.Name,
            "Application name conforms to the required specification.",
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
            "Application version is required but was not provided.",
        )

    } else {

        pass(
            report,
            app.Metadata.Version,
            "Application version conforms to the required specification.",
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
            "At least one organization definition is required.",
        )

    } else {

        pass(
            report,
            fmt.Sprintf(
                "%d organization(s)",
                len(app.Metadata.Catalog.Organization),
            ),
            "Organization information conforms to the required specification.",
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
                "Organization name is required but was not provided.",
            )

        } else {

            pass(
                report,
                org.Name,
                "Organization information conforms to the required specification.",
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
