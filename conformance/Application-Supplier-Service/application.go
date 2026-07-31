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
    )

    if app.APIVersion == "" {
         fail(
            report,
            "apiVersion Key is required With Type=string",
        )   
    } else {

    pass(
        report,
        fmt.Sprintf(
            "Required, Type=string, Value=%s",
            app.APIVersion,
        ),
    )
}

    check(
        report,
        "kind",
    )

    if app.Kind == "" {
         fail(
            report,
            fmt.Sprintf(
                "kind Key is required With Type string and Value will be ApplicationDescription, Found=%s",
                app.Kind,
            ),
        )
    }  else if app.Kind != "ApplicationDescription" {
         fail(
            report,
            fmt.Sprintf(
                "kind Key is required With Type string and Value will be ApplicationDescription, Found=%s",
                app.Kind,
            ),
        )
    } else {

    pass(
        report,
        fmt.Sprintf(
            "Required, Type=string, Expected=ApplicationDescription, Value=%s",
            app.Kind,
        ),
    )
}

    check(
        report,
        "id",
    )
    re := regexp.MustCompile(
        `^[a-z0-9-]{1,200}$`,
    )

    if app.ID == "" {
        fail(
            report,
            "id key is required With Type=string and Value will be lowercase letters, numbers and dashes only, max length=200",
        )
    } else if !re.MatchString(app.ID) {

      fail(
            report,
            fmt.Sprintf(
                "Invalid Id, Followed by With Type string and Value will be lowercase letters, numbers and dashes only, max length=200: %s",
                app.ID,
            ),
        )
    } else {
    pass(
        report,
        fmt.Sprintf(
            "id  (Required, lowercase letters, numbers and dashes only, max length=200) = %s",
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
    )

    if len(app.DeploymentProfile) == 0 {
            fail(report, "deploymentProfiles required")
    }

    componentNames := make(map[string]bool)

    for _, profile :=
        range app.DeploymentProfile {

  if profile.Type == "" {
    fail(
        report,
        "deploymentProfile.type required",
    )
} else if profile.Type != "helm" &&
    profile.Type != "compose" {

    fail(
        report,
        fmt.Sprintf(
            "unsupported deployment type: %s",
            profile.Type,
        ),
    )
}

  if profile.ID == "" {
    fail(
        report,
        "deploymentProfile.id required",
    )
} else {
    pass(
        report,
        fmt.Sprintf(
            "ID=%s, Type=%s",
            profile.ID,
            profile.Type,
        ),
    )
}

        if len(profile.Components) == 0 {

                fail(
                    report,
                    fmt.Sprintf(
                        "deploymentProfile '%s' has no components",
                        profile.ID,
                    ),
                )
        }

        for _, component :=
            range profile.Components {

            check(
                report,
                "component.name",
            )

            if component.Name == "" {
                    fail(
                        report,
                        "component.name required",
                    )
            } else {

            componentNames[
                component.Name,
            ] = true

            pass(
                report,
                fmt.Sprintf(
                    "component.name  (Required, Type=string) = %s",
                    component.Name,
                ),
            )
        }

            if len(component.Properties) == 0 {
                    fail(
                        report,
                        fmt.Sprintf(
                            "component '%s' properties required",
                            component.Name,
                        ),
                    )
            }

            if profile.Type == "helm" {

                check(
                    report,
                    "repository",
                )

                repository, ok :=
                    component.Properties["repository"]

                if !ok {
                        fail(
                            report,
                            fmt.Sprintf(
                                "component '%s' repository required",
                                component.Name,
                            ),
                        )
                } else {

                pass(
                    report,
                    fmt.Sprintf(
                        "repository  (Required, Type=string) = %s",
                        repository,
                    ),
                )
            }

                check(
                    report,
                    "revision",
                )

                revision, ok :=
                    component.Properties["revision"]

                if !ok {
                        fail(
                            report,
                            fmt.Sprintf(
                                "component '%s' revision required",
                                component.Name,
                            ),
                        )
                } else {

                pass(
                    report,
                    fmt.Sprintf(
                        "revision  (Required, Type=string) = %s",
                        revision,
                    ),
                )
            }
            }

            if profile.Type == "compose" {

                check(
                    report,
                    "compose.packageLocation",
                )

                location, ok :=
                    component.Properties["packageLocation"]

                if !ok {

                        fail(
                            report,
                            fmt.Sprintf(
                                "component '%s' packageLocation required",
                                component.Name,
                            ),
                        )
                } else {

                pass(
                    report,
                    fmt.Sprintf(
                        "packageLocation  (Required, Type=string) = %s",
                        location,
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

    for _, schema :=
        range app.Configuration.Schema {

        check(
            report,
            "schema",
        )

        if schema.Name == "" {

            return nil,
                fail(
                    report,
                    "schema name required",
                )
        } else if schema.DataType == "" {

            return nil,
                fail(
                    report,
                    fmt.Sprintf(
                        "schema '%s' datatype required",
                        schema.Name,
                    ),
                )
        } else {

        schemas[schema.Name] = true
        pass(
            report,
            fmt.Sprintf(
                "Name=%s, DataType=%s, AllowEmpty=%t",
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

    for _, section :=
        range app.Configuration.Sections {

        check(
            report,
            "configuration.section",
        )

        pass(
            report,
            fmt.Sprintf(
                "Name=%s",
                section.Name,
            ),
        )

        for _, setting :=
            range section.Settings {

            check(
                report,
                fmt.Sprintf(
                    "setting.parameter.%s",
                    setting.Parameter,
                ),
            )

            if _, ok :=
                app.Parameters[
                    setting.Parameter]; !ok {

                return fail(
                    report,
                    fmt.Sprintf(
                        "configuration references unknown parameter '%s'",
                        setting.Parameter,
                    ),
                )
            }

            pass(
                report,
                fmt.Sprintf(
                    "Parameter=%s",
                    setting.Parameter,
                ),
            )

            if setting.Schema != "" {

                if !schemas[
                    setting.Schema] {

                    return fail(
                        report,
                        fmt.Sprintf(
                            "configuration references unknown schema '%s'",
                            setting.Schema,
                        ),
                    )
                }

                pass(
                    report,
                    fmt.Sprintf(
                        "schema '%s' found",
                        setting.Schema,
                    ),
                )
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
                )

                if !componentNames[
                    component] {

                    fail(
                        report,
                        fmt.Sprintf(
                            "parameter '%s' references unknown component '%s'",
                            parameterName,
                            component,
                        ),
                    )
                } else {

                pass(
                    report,
                    fmt.Sprintf(
                        "Pointer=%s, Component=%s",
                        target.Pointer,
                        component,
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
    )

    if app.Metadata.Name == "" {

        fail(
            report,
            "metadata.name required",
        )
    } else {         
    pass(
        report,
        fmt.Sprintf(
            "metadata.name  (Required, Type=string) = %s",
            app.Metadata.Name,
        ),
    )
}

    check(
        report,
        "metadata.version",
    )

    if app.Metadata.Version == "" {

        fail(
            report,
            "metadata.version required",
        )
    } else {
        pass(
        report,
        fmt.Sprintf(
            "metadata.version  (Required, Type=string) = %s",
            app.Metadata.Version,
        ),
    )
}

    // check(
    //     report,
    //     "metadata.catalog.organization",
    // )

    if len(
        app.Metadata.Catalog.Organization,
    ) == 0 {

        fail(
            report,
            "metadata.catalog.organization required",
        )
    }

    for _, org :=
        range app.Metadata.Catalog.Organization {

        check(
            report,
            "metadata.catalog.organization.name",
        )

        if org.Name == "" {

             fail(
                report,
                "organization.name required",
            )
        } else {

        pass(
            report,
            fmt.Sprintf(
                "organization.name  (Required, Type=string) = %s",
                org.Name,
            ),
        )
    }
    }

    return nil
}

func check(report *ValidationReport, msg string) {
    fmt.Println("Validate", msg)
    report.Check(msg)
}

func pass(report *ValidationReport, msg string) {
    fmt.Println("PASS", msg)
    report.Pass(msg)
}

func fail(report *ValidationReport, msg string) error {
    fmt.Println("FAIL", msg)
    report.Fail(msg)
    return fmt.Errorf(msg)
}