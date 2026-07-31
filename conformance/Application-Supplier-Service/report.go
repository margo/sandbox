package main

import (
    "html/template"
    "os"
)

type ValidationEntry struct {
    Validate string
    Details  string
    Status   string
}

type ValidationReport struct {
    Entries []ValidationEntry
    Status  string
    ApplicationName string
}

const reportTemplate = `
<!DOCTYPE html>
<html>

<head>
<meta charset="UTF-8">
<title>Application Conformance Report</title>

<style>

body {
    font-family: Arial, sans-serif;
    margin: 20px;
    background: #f5f5f5;
}

.header {
    background: #333;
    color: white;
    padding: 20px;
    border-radius: 5px;
}

.summary {
    margin: 20px 0;
    background: white;
    padding: 15px;
    border-radius: 5px;
}

.pass {
    color: green;
    font-weight: bold;
}

.fail {
    color: red;
    font-weight: bold;
}

.validate {
    color: #007bff;
    font-weight: bold;
}

table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 20px;
    background: white;
}

th,
td {
    padding: 10px;
    text-align: left;
    border-bottom: 1px solid #ddd;
}

th {
    background: #f2f2f2;
}

.result-pass {
    color: green;
    font-weight: bold;
}

.result-fail {
    color: red;
    font-weight: bold;
}

</style>

</head>

<body>

<div class="header">

    <h1>Application Conformance Report</h1>

    <p>
        <strong>Application:</strong>
        {{.ApplicationName}}
    </p>

</div>

<div class="summary">

    <h2>Validation Summary</h2>

    <p>
        <strong>Total Entries:</strong>
        {{len .Entries}}
    </p>

    <p>
        <strong>Final Result:</strong>

        {{if eq .Status "PASSED"}}
            <span class="result-pass">✅ PASSED</span>
        {{else}}
            <span class="result-fail">❌ FAILED</span>
        {{end}}

    </p>

</div>

<table>

<tr>
    <th>Validate</th>
    <th>Status</th>
    <th>Details</th>
</tr>

{{range .Entries}}

<tr>

    <td>{{.Validate}}</td>

    <td class="{{lower .Status}}">

        {{if eq .Status "PASS"}}
            ✅ PASS
        {{else if eq .Status "FAIL"}}
            ❌ FAIL
        {{else}}
            -
        {{end}}

    </td>

    <td>{{.Details}}</td>

</tr>

{{end}}

</table>

</body>
</html>
`

func NewValidationReport() *ValidationReport {
    return &ValidationReport{
        Status: "PASSED",
    }
}

func (r *ValidationReport) Log(
    validate string,
    details string,
    status string,
) {

    r.Entries = append(
        r.Entries,
        ValidationEntry{
            Validate: validate,
            Details:  details,
            Status:   status,
        },
    )

    if status == "FAIL" {
        r.Status = "FAILED"
    }
}

func (r *ValidationReport) Check(msg string) {
    r.Entries = append(
        r.Entries,
        ValidationEntry{
            Validate: msg,
        },
    )
}


func (r *ValidationReport) Pass(msg string) {

    if len(r.Entries) > 0 {

        r.Entries[len(r.Entries)-1].Status = "PASS"
        r.Entries[len(r.Entries)-1].Details = msg

        return
    }
}

func (r *ValidationReport) Fail(msg string) {

    if len(r.Entries) > 0 {

        r.Entries[len(r.Entries)-1].Status = "FAIL"
        r.Entries[len(r.Entries)-1].Details = msg

    }

    r.Status = "FAILED"
}



func (r *ValidationReport) GenerateHTMLReport(file string) error {
    funcMap := template.FuncMap{
        "lower": func(s string) string {
            switch s {
            case "PASS":
                return "pass"
            case "FAIL":
                return "fail"
            default:
                return "Validate"
            }
        },
    }

    tmpl := template.Must(
        template.New("report").
            Funcs(funcMap).
            Parse(reportTemplate),
    )

    f, err := os.Create(file)
    if err != nil {
        return err
    }

    defer f.Close()

    return tmpl.Execute(f, r)
}