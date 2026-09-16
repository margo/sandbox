// sandbox/standard/generatedCode/wfm/sbi/problem_helpers.go
// Hand-written helpers for the generated ProblemDetail type. DO NOT regenerate.
package sbi

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strconv"
)

// ── Margo-reserved problem type URIs ─────────────────────────────────────────
// Source: https://docs.margo.org/specification/problem-types
// Stable, unversioned identifiers. Clients MUST use type URI for programmatic
// error handling — NOT the status code or title.
const (
    ProblemBaseURI = "https://docs.margo.org/specification/problem-types"

    // 400 — Malformed request body.
    ProblemTypeInvalidRequest = ProblemBaseURI + "#invalid-request"

    // 403 — Request not authorized by WFM local policy.
    ProblemTypeNotAuthorized = ProblemBaseURI + "#not-authorized"

    // 404 — No gateway found for the given child-device deviceId.
    ProblemTypeGatewayNotFound = ProblemBaseURI + "#gateway-not-found"

    // 404 — No device with the given deviceId found for the client.
    ProblemTypeDeviceNotFound = ProblemBaseURI + "#device-not-found"

    // 404 — Bundle not found for the given digest.
    ProblemTypeInvalidBundle = ProblemBaseURI + "#invalid-bundle"

    // 404 — Deployment not found for the given digest.
    ProblemTypeDeploymentNotFound = ProblemBaseURI + "#deployment-not-found"

    // 404 — Trust domain discovery document not available.
    ProblemTypeDiscoveryDocumentNotFound = ProblemBaseURI + "#discovery-document-not-found"

    // 404 — SPIFFE bundle unavailable.
    ProblemTypeSpiffeBundleNotFound = ProblemBaseURI + "#spiffe-bundle-not-found"

    // 406 — Server cannot generate a response matching the Accept header.
    ProblemTypeServerCannotGenerateResponse = ProblemBaseURI + "#server-cannot-generate-response"

    // 422 — Request body syntactically valid but contains a semantic error.
    ProblemTypeSemanticError = ProblemBaseURI + "#semantic-error"
)

// ── Non-Margo (about:blank) ───────────────────────────────────────────────────
// Used when no Margo-specific problem type applies (RFC 9457 §4.2).
const (
    ProblemTypeAboutBlank = "about:blank"
)

// ProblemContentType is the RFC 9457 media type.
const ProblemContentType = "application/problem+json"

// ── error interface ───────────────────────────────────────────────────────────

func (p *ProblemDetail) Error() string {
    if p.Detail != nil && *p.Detail != "" {
        return fmt.Sprintf("[%d] %s: %s", p.Status, p.Title, *p.Detail)
    }
    return fmt.Sprintf("[%d] %s", p.Status, p.Title)
}

func (p *ProblemDetail) IsRetryable() bool {
    return p.Retryable != nil && *p.Retryable
}

func (p *ProblemDetail) ShouldRetry() bool {
    return p.IsRetryable() || p.Status >= 500
}

// ── Builder ───────────────────────────────────────────────────────────────────

func NewProblemDetail(problemType, title string, status int) *ProblemDetail {
    return &ProblemDetail{Type: problemType, Title: title, Status: status}
}

func (p *ProblemDetail) WithDetail(d string) *ProblemDetail {
    p.Detail = &d
    return p
}

func (p *ProblemDetail) WithInstance(i string) *ProblemDetail {
    p.Instance = &i
    return p
}

func (p *ProblemDetail) WithRetryable(r bool) *ProblemDetail {
    p.Retryable = &r
    return p
}

func (p *ProblemDetail) WithRetryAfterSeconds(s int) *ProblemDetail {
    p.RetryAfterSeconds = &s
    return p
}

func (p *ProblemDetail) WithBackoffStrategy(s ProblemDetailBackoffStrategy) *ProblemDetail {
    p.BackoffStrategy = &s
    return p
}

// ── Convenience constructors ──────────────────────────────────────────────────

func NewInvalidRequest(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeInvalidRequest, "Invalid Request", http.StatusBadRequest).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewSemanticError(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeSemanticError, "Semantic Error", http.StatusUnprocessableEntity).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewNotAuthorized(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeNotAuthorized, "Not Authorized", http.StatusForbidden).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewGatewayNotFound(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeGatewayNotFound, "Gateway Not Found", http.StatusNotFound).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewDeviceNotFound(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeDeviceNotFound, "Device Not Found", http.StatusNotFound).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewInvalidBundle(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeInvalidBundle, "Invalid Bundle", http.StatusNotFound).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewDeploymentNotFound(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeDeploymentNotFound, "Deployment Not Found", http.StatusNotFound).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewServerCannotGenerateResponse(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeServerCannotGenerateResponse, "Server Cannot Generate Response", http.StatusNotAcceptable).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewInternalError(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeAboutBlank, "Internal Server Error", http.StatusInternalServerError).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(true).WithBackoffStrategy(Exponential)
}

func NewServiceUnavailable(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeAboutBlank, "Service Unavailable", http.StatusServiceUnavailable).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(true).WithBackoffStrategy(Exponential)
}

func NewConflict(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeAboutBlank, "Conflict", http.StatusConflict).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

func NewTooManyRequests(detail, instance string, retryAfterSeconds int) *ProblemDetail {
    return NewProblemDetail(ProblemTypeAboutBlank, "Too Many Requests", http.StatusTooManyRequests).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(true).WithBackoffStrategy(Exponential).
        WithRetryAfterSeconds(retryAfterSeconds)
}

func NewNotImplemented(detail, instance string) *ProblemDetail {
    return NewProblemDetail(ProblemTypeAboutBlank, "Not Implemented", http.StatusNotImplemented).
        WithDetail(detail).WithInstance(instance).
        WithRetryable(false).WithBackoffStrategy(None)
}

// ── HTTP writer ───────────────────────────────────────────────────────────────

func (p *ProblemDetail) WriteHTTP(w http.ResponseWriter) {
    b, err := p.MarshalJSON()
    if err != nil {
        http.Error(w, http.StatusText(http.StatusInternalServerError),
            http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", ProblemContentType)
    if p.RetryAfterSeconds != nil {
        w.Header().Set("Retry-After", strconv.Itoa(*p.RetryAfterSeconds))
    }
    w.WriteHeader(p.Status)
    _, _ = w.Write(b)
}

// ── Client-side helpers ───────────────────────────────────────────────────────

func ParseErrorResponse(resp *http.Response) error {
    if resp == nil {
        return nil
    }
    if resp.StatusCode == http.StatusNotModified ||
        (resp.StatusCode >= 200 && resp.StatusCode < 300) {
        return nil
    }
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("HTTP %d: failed to read error body: %w", resp.StatusCode, err)
    }
    if resp.Header.Get("Content-Type") == ProblemContentType {
        var pd ProblemDetail
        if jsonErr := json.Unmarshal(body, &pd); jsonErr == nil {
            return &pd
        }
    }
    return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func AsProblemDetail(err error) (*ProblemDetail, bool) {
    var pd *ProblemDetail
    return pd, errors.As(err, &pd)
}
