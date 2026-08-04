package main

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "time"
    "gopkg.in/yaml.v3"
)

func main() {

    if len(os.Args) < 2 {

        fmt.Println(
            "Usage: validator <folder|zip|tar.gz>",
        )

        os.Exit(1)
    }

    inputPath := os.Args[1]

    var workDir string

    info, err := os.Stat(inputPath)

    if err != nil {
        panic(err)
    }

    //--------------------------------------------------
    // Folder
    //--------------------------------------------------

    if info.IsDir() {

        workDir = inputPath

    } else if filepath.Ext(inputPath) == ".zip" {

        workDir = "./workdir"

        os.RemoveAll(workDir)

        err = ExtractZip(
            inputPath,
            workDir,
        )

        if err != nil {
            panic(err)
        }

    } else if filepath.Ext(inputPath) == ".gz" {

        workDir = "./workdir"

        os.RemoveAll(workDir)

        err = ExtractTarGz(
            inputPath,
            workDir,
        )

        if err != nil {
            panic(err)
        }

    } else {

        fmt.Println(
            "Unsupported input type",
        )

        os.Exit(1)
    }

    fmt.Println(
        "Extracted Path:",
        workDir,
    )

    //--------------------------------------------------
    // Find Application Description
    //--------------------------------------------------

    appYaml, err :=
        FindApplicationDescription(
            workDir,
        )

    if err != nil {
        panic(err)
    }

    fmt.Println(
        "Application Description:",
        appYaml,
    )

    //--------------------------------------------------
    // Read YAML
    //--------------------------------------------------

    data, err :=
        os.ReadFile(appYaml)

    if err != nil {
        panic(err)
    }

    //--------------------------------------------------
    // Handle Template Variables
    //--------------------------------------------------
    //
    // Example:
    //
    // repository: {{HELM_REPOSITORY}}
    // revision: {{CHART_VERSION}}
    //
    //--------------------------------------------------

    content := string(data)

    placeholderRegex :=
        regexp.MustCompile(`\{\{[^}]+\}\}`)

    content =
        placeholderRegex.ReplaceAllStringFunc(
            content,
            func(s string) string {

                return "\"" + s + "\""
            },
        )

    data = []byte(content)

    //--------------------------------------------------
    // Parse YAML
    //--------------------------------------------------

    var app ApplicationDescription

    err = yaml.Unmarshal(
        data,
        &app,
    )

    appName := app.Metadata.Name

if appName == "" {
    appName = filepath.Base(workDir)
}

reportFile := filepath.Join(
    "/home/margo/rishab/sandbox/conformance/Runner/application-supplier",
    fmt.Sprintf(
        "%s_%s.html",
        appName,
        time.Now().Format("02-01-2006_15-04-05"),
    ),
)

    if err != nil {

        fmt.Println(
            "\nYAML PARSE FAILED",
        )

        fmt.Println(
            "File:",
            appYaml,
        )

        fmt.Println(
            "Error:",
            err,
        )

        os.Exit(1)
    }
//--------------------------------------------------
// Application Validation
//--------------------------------------------------

report := NewValidationReport()

report.ApplicationName = app.Metadata.Name
report.ApplicationVersion = app.Metadata.Version

if report.ApplicationName == "" {
    report.ApplicationName = app.ID
}

err = ValidateAppDescription(
    &app,
    report,
)

if err != nil {

    report.Status = "FAILED"

_ = report.GenerateHTMLReport(
    reportFile,
)

    fmt.Println(
        "\nVALIDATION FAILED:",
    )

    fmt.Println(err)

    os.Exit(1)
}

fmt.Println(
    "\nApplication Description Validation PASSED",
)

    //--------------------------------------------------
    // Deployment Profile Validation
    //--------------------------------------------------

    for _, profile :=
        range app.DeploymentProfile {

        fmt.Println(
            "\nDeployment Type:",
            profile.Type,
        )

        switch profile.Type {

        //--------------------------------------------------
        // HELM
        //--------------------------------------------------

        case "helm":

            for _, component :=
                range profile.Components {

report.Check(
    fmt.Sprintf(
        "Validating Helm component '%s'",
        component.Name,
    ),
    "network",
    "Helm repository reachable",
)

              err := ValidateHelmComponent(
    component,
)

if err != nil {

report.Fail(
    "unreachable",
    fmt.Sprintf(
        "Helm validation failed for component '%s' : %v",
        component.Name,
        err,
    ),
)

    fmt.Println(
        "FAIL:",
        err,
    )

    continue
}

report.Pass(
    "reachable",
    fmt.Sprintf(
        "Helm repository reachable for component '%s'",
        component.Name,
    ),
)

fmt.Println(
    "PASS: Helm Repository Reachable",
)
            }

        //--------------------------------------------------
        // COMPOSE
        //--------------------------------------------------

        case "compose":

            for _, component :=
                range profile.Components {

                location :=
                    component.Properties["packageLocation"].(string)

               report.Check(
    "compose.packageLocation",
    "network",
    "Package location reachable",
)

               err := CheckPackageLocation(
    location,
)

if err != nil {

    report.Fail(
    "unreachable",
    fmt.Sprintf(
        "packageLocation unreachable for component '%s' : %v",
        component.Name,
        err,
    ),
)

    fmt.Println(
        "FAIL:",
        err,
    )

    continue
}

report.Pass(
    location,
    fmt.Sprintf(
        "packageLocation reachable for component '%s'",
        component.Name,
    ),
)

fmt.Println(
    "PASS: packageLocation reachable",
)
            }

        default:

            fmt.Printf(
                "Unsupported deployment type %s\n",
                profile.Type,
            )
        }
    }

    fmt.Println(
    "\nConformance Validation Completed",
)

err = report.GenerateHTMLReport(
    reportFile,
)

reportPath, _ := filepath.Abs(
    reportFile,
)

fmt.Println()
fmt.Println("=====================================")
fmt.Println("Validation Report Generated")
fmt.Println("Report Path:", reportPath)
fmt.Println("Validation Result :", report.Status)

fmt.Println("=====================================")
fmt.Println()

if err != nil {

    fmt.Println(
        "failed to generate report:",
        err,
    )
}
}