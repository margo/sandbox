package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	htmsighttp "github.com/lestrrat-go/htmsig/http"
)

const (
	WFMPort          = ":3001"
	ClientsFile      = "./data/clients.json"
	DeploymentsFile  = "./data/deployments.json"
)

// Global state
var (
	clients          = make(map[string]ClientData)
	deployments      = make(map[string]DeploymentData)
	mu               sync.RWMutex
	assertionsConfig *AssertionsConfig
)

// Assertions loaded from JSON (data-driven validation)
type AssertionsConfig struct {
	Endpoints            map[string]EndpointAssertion `json:"endpoints"`
	ErrorResponses       map[string]ErrorResponseSpec `json:"error_responses"`
	RejectedCertificates []string                     `json:"rejected_certificates,omitempty"`
}

type EndpointAssertion struct {
	Path               string                 `json:"path"`
	Method             string                 `json:"method"`
	StatusCode         int                    `json:"status_code"`
	ValidationErrorKey string                 `json:"validation_error_key,omitempty"`
	Validations        []ValidationRule       `json:"validations"`
	ResponseStructure  map[string]interface{} `json:"response_structure"`
}

type ValidationRule struct {
	RuleID      string      `json:"rule_id"`
	Field       string      `json:"field"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Value       interface{} `json:"value,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	MinLength   int         `json:"minLength,omitempty"`
	MinItems    int         `json:"minItems,omitempty"`
	ItemsType   string      `json:"itemsType,omitempty"`
	ItemsEnum   []string    `json:"itemsEnum,omitempty"`
	Description string      `json:"description"`
}

type ErrorResponseSpec struct {
	StatusCode int    `json:"status_code"`
	Format     string `json:"format"`
	Status     string `json:"status,omitempty"`
}

// Data structures per Margo spec
type ClientData struct {
	ID              string                 `json:"id"`
	Certificate     string                 `json:"certificate"`
	OnboardedAt     time.Time              `json:"onboarded_at"`
	Capabilities    map[string]interface{} `json:"capabilities,omitempty"`
	DeploymentsData []string               `json:"deployments,omitempty"`
}

type DeploymentData struct {
	ID            string        `json:"id"`
	ClientID      string        `json:"client_id"`
	StatusHistory []interface{} `json:"status_history"`
}

type ValidationError struct {
	RuleID string `json:"rule_id"`
	Error  string `json:"error"`
}

type ResponseError struct {
	Status    string            `json:"status,omitempty"`
	Errors    []ValidationError `json:"errors,omitempty"`
	Error     string            `json:"error,omitempty"`
	Message   string            `json:"message,omitempty"`
	ClientID  string            `json:"clientId,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
}

// ===== VALIDATION ENGINE (Reads from assertions.json) =====

func loadAssertions(filePath string) (*AssertionsConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read assertions file: %w", err)
	}

	var config AssertionsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse assertions JSON: %w", err)
	}

	log.Printf("✓ Loaded assertions from: %s", filePath)
	return &config, nil
}

// validateRequest applies rules from assertions.json to incoming request
func validateRequest(endpointKey string, body map[string]interface{}) []ValidationError {
	endpoint, exists := assertionsConfig.Endpoints[endpointKey]
	if !exists {
		log.Printf("⚠ No assertions found for endpoint: %s", endpointKey)
		return []ValidationError{}
	}

	var errors []ValidationError

	// Apply each validation rule from assertions
	for _, rule := range endpoint.Validations {
		if err := applyRule(rule, body); err != nil {
			errors = append(errors, *err)
		}
	}

	return errors
}

func applyRule(rule ValidationRule, body map[string]interface{}) *ValidationError {
	fieldValues, exists := getFieldValues(body, rule.Field)

	// Check if required field is missing
	if rule.Required && !exists {
		return &ValidationError{
			RuleID: rule.RuleID,
			Error:  fmt.Sprintf("%s is required", rule.Field),
		}
	}

	if !exists {
		return nil // Field not required and not present - OK
	}

	for _, fieldValue := range fieldValues {
		switch rule.Type {
		case "string":
			strValue, ok := fieldValue.(string)
			if !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be a string, got %T", rule.Field, fieldValue),
				}
			}

			if rule.MinLength > 0 && len(strValue) < rule.MinLength {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be at least %d characters", rule.Field, rule.MinLength),
				}
			}

			if rule.Value != nil && strValue != rule.Value {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be '%v', got '%s'", rule.Field, rule.Value, strValue),
				}
			}

			if len(rule.Enum) > 0 && !containsString(rule.Enum, strValue) {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be one of %v, got '%s'", rule.Field, rule.Enum, strValue),
				}
			}

		case "array":
			arr, ok := fieldValue.([]interface{})
			if !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be an array, got %T", rule.Field, fieldValue),
				}
			}
			if rule.MinItems > 0 && len(arr) < rule.MinItems {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must have at least %d items, got %d", rule.Field, rule.MinItems, len(arr)),
				}
			}
			for _, item := range arr {
				if err := validateArrayItem(rule, item); err != nil {
					return err
				}
			}

		case "object":
			if _, ok := fieldValue.(map[string]interface{}); !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be an object, got %T", rule.Field, fieldValue),
				}
			}

		case "number":
			if _, ok := fieldValue.(float64); !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s must be a number, got %T", rule.Field, fieldValue),
				}
			}
		}
	}

	return nil
}

func getFieldValue(data interface{}, path string) (interface{}, bool) {
	values, exists := getFieldValues(data, path)
	if !exists || len(values) == 0 {
		return nil, false
	}
	return values[0], true
}

func getFieldValues(data interface{}, path string) ([]interface{}, bool) {
	if path == "" {
		return []interface{}{data}, true
	}

	values, missing := collectFieldValues(data, strings.Split(path, "."))
	if missing {
		return nil, false
	}
	return values, true
}

func collectFieldValues(data interface{}, parts []string) ([]interface{}, bool) {
	if len(parts) == 0 {
		return []interface{}{data}, false
	}

	part := parts[0]
	switch typed := data.(type) {
	case map[string]interface{}:
		next, exists := typed[part]
		if !exists {
			return nil, true
		}
		return collectFieldValues(next, parts[1:])
	case []interface{}:
		if part == "*" {
			if len(typed) == 0 {
				return nil, false
			}

			var values []interface{}
			missing := false
			for _, item := range typed {
				itemValues, itemMissing := collectFieldValues(item, parts[1:])
				if itemMissing {
					missing = true
				}
				values = append(values, itemValues...)
			}
			return values, missing
		}

		index := -1
		if _, err := fmt.Sscanf(part, "%d", &index); err != nil || index < 0 || index >= len(typed) {
			return nil, true
		}
		return collectFieldValues(typed[index], parts[1:])
	default:
		return nil, true
	}
}

func validateArrayItem(rule ValidationRule, item interface{}) *ValidationError {
	if rule.ItemsType != "" {
		switch rule.ItemsType {
		case "string":
			if _, ok := item.(string); !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s items must be strings, got %T", rule.Field, item),
				}
			}
		case "object":
			if _, ok := item.(map[string]interface{}); !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s items must be objects, got %T", rule.Field, item),
				}
			}
		case "number":
			if _, ok := item.(float64); !ok {
				return &ValidationError{
					RuleID: rule.RuleID,
					Error:  fmt.Sprintf("%s items must be numbers, got %T", rule.Field, item),
				}
			}
		}
	}

	if len(rule.ItemsEnum) > 0 {
		strValue, ok := item.(string)
		if !ok || !containsString(rule.ItemsEnum, strValue) {
			return &ValidationError{
				RuleID: rule.RuleID,
				Error:  fmt.Sprintf("%s items must be one of %v, got '%v'", rule.Field, rule.ItemsEnum, item),
			}
		}
	}

	return nil
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func validationErrorResponse(endpointKey string, errors []ValidationError) (int, interface{}) {
	endpoint, exists := assertionsConfig.Endpoints[endpointKey]
	if !exists || endpoint.ValidationErrorKey == "" {
		return 422, ResponseError{Status: "validation_failed", Errors: errors}
	}

	spec, exists := assertionsConfig.ErrorResponses[endpoint.ValidationErrorKey]
	if !exists {
		return 422, ResponseError{Status: "validation_failed", Errors: errors}
	}

	if spec.Format == "error_string" {
		return spec.StatusCode, ResponseError{Error: errors[0].Error}
	}

	status := spec.Status
	if status == "" {
		status = "validation_failed"
	}
	return spec.StatusCode, ResponseError{
		Status: status,
		Errors: errors,
	}
}

func isRejectedCertificate(certificate string) bool {
	return containsString(assertionsConfig.RejectedCertificates, certificate)
}

func validateContentDigest(body []byte, headerValue string) bool {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return false
	}

	sum := sha256.Sum256(body)
	expectedBase64 := base64.StdEncoding.EncodeToString(sum[:])

	if headerValue == expectedBase64 {
		return true
	}

	if strings.HasPrefix(strings.ToLower(headerValue), "sha-256=") {
		digestValue := strings.TrimSpace(headerValue[len("sha-256="):])
		digestValue = strings.Trim(digestValue, ":")
		return digestValue == expectedBase64
	}

	return false
}

func loadServerCACertificate() (string, error) {
	caCertPath := filepath.Join("certs", "ca-cert.pem")
	data, err := os.ReadFile(caCertPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeCertificateString(cert string) string {
	cert = strings.TrimSpace(cert)
	if strings.Contains(cert, "-----BEGIN CERTIFICATE-----") {
		lines := strings.Split(cert, "\n")
		var normalized strings.Builder
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "-----BEGIN") || strings.HasPrefix(line, "-----END") {
				continue
			}
			normalized.WriteString(line)
		}
		return normalized.String()
	}
	return strings.ReplaceAll(cert, "\n", "")
}

// ===== RFC 9421 HTTP MESSAGE SIGNATURE VERIFICATION =====

// replayWindow is how far into the past or future a `created` timestamp may be.
const replayWindow = 5 * time.Minute

// SignatureInputParams holds parsed fields from the Signature-Input header.
// e.g. sig1=("@method" "@target-uri" "content-digest");created=1680575171;keyid="my-key"
type SignatureInputParams struct {
	Label      string   // "sig1"
	Components []string // ["@method", "@target-uri", "content-digest"]
	Created    int64    // Unix timestamp
	KeyID      string
}

// parseSignatureInput parses the first sig label in the Signature-Input header.
func parseSignatureInput(headerValue string) (*SignatureInputParams, error) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return nil, fmt.Errorf("Signature-Input header is empty")
	}

	// Find label and rest: "sig1=(..."
	eqIdx := strings.Index(headerValue, "=(")
	if eqIdx < 0 {
		return nil, fmt.Errorf("Signature-Input malformed: no '=(' found")
	}
	label := strings.TrimSpace(headerValue[:eqIdx])

	rest := headerValue[eqIdx+1:]

	// Extract the component list inside (...)
	closeIdx := strings.Index(rest, ")")
	if closeIdx < 0 {
		return nil, fmt.Errorf("Signature-Input malformed: no closing ')' found")
	}
	componentStr := rest[1:closeIdx] // strip outer parens
	params := rest[closeIdx+1:]      // ";created=...;keyid=..."

	// Parse component identifiers: strip quotes and spaces
	var components []string
	for _, raw := range strings.Fields(componentStr) {
		comp := strings.Trim(raw, `"`)
		components = append(components, comp)
	}

	// Parse key=value pairs after the component list
	si := &SignatureInputParams{Label: label, Components: components}
	for _, kv := range strings.Split(params, ";") {
		kv = strings.TrimSpace(kv)
		if strings.HasPrefix(kv, "created=") {
			val := strings.TrimPrefix(kv, "created=")
			if _, err := fmt.Sscanf(val, "%d", &si.Created); err != nil {
				return nil, fmt.Errorf("Signature-Input: invalid created value: %v", val)
			}
		} else if strings.HasPrefix(kv, "keyid=") {
			si.KeyID = strings.Trim(strings.TrimPrefix(kv, "keyid="), `"`)
		}
	}

	if si.Created == 0 {
		return nil, fmt.Errorf("Signature-Input: missing 'created' parameter")
	}
	return si, nil
}

// extractSigValue extracts the base64 value for a given label from the Signature header.
// e.g. "sig1=:<base64>:" -> base64 string
func extractSigValue(sigHeader, label string) ([]byte, error) {
	// Look for "label=:<b64>:"
	prefix := label + "=:"
	idx := strings.Index(sigHeader, prefix)
	if idx < 0 {
		return nil, fmt.Errorf("Signature header: label '%s' not found", label)
	}
	rest := sigHeader[idx+len(prefix):]
	endIdx := strings.Index(rest, ":")
	if endIdx < 0 {
		return nil, fmt.Errorf("Signature header: closing ':' not found for label '%s'", label)
	}
	b64 := rest[:endIdx]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("Signature header: base64 decode failed: %w", err)
	}
	return decoded, nil
}

// buildSignatureBase reconstructs the canonical signature base string per RFC 9421.
func buildSignatureBase(r *http.Request, components []string, sigInputHeader string) string {
	var sb strings.Builder
	for _, comp := range components {
		switch comp {
		case "@method":
			sb.WriteString(fmt.Sprintf("@method: %s\n", r.Method))
		case "@target-uri":
			scheme := "https"
			host := r.Host
			if host == "" {
				host = "localhost:3001"
			}
			uri := fmt.Sprintf("%s://%s%s", scheme, host, r.RequestURI)
			sb.WriteString(fmt.Sprintf("@target-uri: %s\n", uri))
		case "@authority":
			// RFC 9421: authority is the host[:port] component
			host := r.Host
			if host == "" {
				host = "localhost:3001"
			}
			sb.WriteString(fmt.Sprintf("@authority: %s\n", host))
		case "@request-target":
			sb.WriteString(fmt.Sprintf("@request-target: %s %s\n", strings.ToLower(r.Method), r.RequestURI))
		default:
			// Treat as HTTP header name (lowercase) - no quotes per RFC 9421
			val := r.Header.Get(comp)
			sb.WriteString(fmt.Sprintf("%s: %s\n", strings.ToLower(comp), val))
		}
	}
	// Append @signature-params line: RFC 9421 §2.5 requires only the value part (without "label=")
	// Strip the "label=" prefix from sigInputHeader to get just ("@method" ...);created=...
	sigParamsValue := sigInputHeader
	if eqIdx := strings.Index(sigInputHeader, "=("); eqIdx >= 0 {
		sigParamsValue = sigInputHeader[eqIdx+1:]
	}
	sb.WriteString(fmt.Sprintf("@signature-params: %s", sigParamsValue))
	return sb.String()
}

// verifyRFC9421Signature performs full RFC 9421 signature verification using
// the htmsig library (same library the device agent uses for signing).
// certPEM is the PEM-encoded X.509 certificate (or base64-encoded PEM) of the signer.
// bodyBytes may be nil for GET requests.
func verifyRFC9421Signature(r *http.Request, certPEM string, bodyBytes []byte) error {
	// 1. Signature-Input must be present — if not, it's a missing signature (401)
	if r.Header.Get("Signature-Input") == "" {
		return fmt.Errorf("Signature missing: Signature-Input header not present")
	}

	// 2. Validate Content-Digest if body is present (must come after Signature-Input
	//    presence check, but before htmsig verification — empty/missing digest → 400)
	if len(bodyBytes) > 0 {
		digestHeader := r.Header.Get("Content-Digest")
		if digestHeader == "" || !validateContentDigest(bodyBytes, digestHeader) {
			return fmt.Errorf("Content-Digest mismatch or missing")
		}
	}

	// 3. Extract public key from stored certificate
	publicKey, err := extractPublicKeyFromCertPEM(certPEM)
	if err != nil {
		return fmt.Errorf("failed to extract public key from client cert: %w", err)
	}

	// 4. Use htmsig library verifier with a static key resolver
	resolver := htmsighttp.StaticKeyResolver(publicKey)
	verifier := htmsighttp.NewVerifier(resolver, htmsighttp.WithValidateExpires(false))
	if err := verifier.VerifyRequest(context.Background(), r); err != nil {
		return fmt.Errorf("RFC9421 signature verification failed: %w", err)
	}
	return nil
}

// extractPublicKeyFromCertPEM parses X.509 certificate in various formats:
// - Base64-encoded PEM (device-agent sends this in JSON)
// - Plain PEM format
// - Base64-encoded DER (raw binary)
func extractPublicKeyFromCertPEM(certPEM string) (interface{}, error) {
	var certBytes []byte
	
	// Try 1: Decode base64 first
	decoded, err := base64.StdEncoding.DecodeString(certPEM)
	if err == nil {
		// Base64 decode succeeded - check if result is PEM or DER
		block, _ := pem.Decode(decoded)
		if block != nil {
			// It was base64-encoded PEM (device-agent format)
			certBytes = block.Bytes
		} else {
			// It was base64-encoded DER
			certBytes = decoded
		}
	} else {
		// Base64 decode failed - try PEM decode directly
		block, _ := pem.Decode([]byte(certPEM))
		if block != nil {
			certBytes = block.Bytes
		} else {
			return nil, fmt.Errorf("cert is not base64, PEM, or valid format")
		}
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X.509 certificate: %w", err)
	}
	return cert.PublicKey, nil
}

// signatureAuthError returns a spec-compliant 401 response body.
func signatureAuthError(msg string) ResponseError {
	return ResponseError{
		Error:   "Signature verification failed. Ensure you are signing with the correct X.509 private key.",
		Message: msg,
	}
}

func deploymentKey(clientID, deploymentID string) string {
	return clientID + ":" + deploymentID
}

const defaultDeploymentID = "a3e2f5dc-912e-494f-8395-52cf3769bc06"

func ensureDefaultDeployment(clientID string) {
	key := deploymentKey(clientID, defaultDeploymentID)
	if _, exists := deployments[key]; exists {
		return
	}

	deployments[key] = DeploymentData{
		ID:            defaultDeploymentID,
		ClientID:      clientID,
		StatusHistory: []interface{}{},
	}
}

func quoteETag(value string) string {
	return `"` + value + `"`
}

func normalizeETag(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func buildDeploymentYAML(clientID, deploymentID string) []byte {
	_ = clientID
	templateBytes, err := os.ReadFile("manifests/deployment-template.yaml")
	if err != nil {
		log.Printf("[DeploymentYAML] Warning: could not read deployment-template.yaml: %v — using empty manifest", err)
		return []byte{}
	}
	result := strings.ReplaceAll(string(templateBytes), "{{deploymentId}}", deploymentID)
	return []byte(result)
}

func buildBundleArchive(clientID string, deploymentIDs []string) ([]byte, error) {
	var archive bytes.Buffer

	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, deploymentID := range deploymentIDs {
		content := buildDeploymentYAML(clientID, deploymentID)
		header := &tar.Header{
			Name: fmt.Sprintf("%s.yaml", deploymentID),
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tarWriter.Write(content); err != nil {
			return nil, err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return archive.Bytes(), nil
}

func buildStateManifest(clientID string, deploymentIDs []string) (map[string]interface{}, string, error) {
	refs := make([]interface{}, 0, len(deploymentIDs))
	bundle := interface{}(nil)

	if len(deploymentIDs) > 0 {
		bundleBytes, err := buildBundleArchive(clientID, deploymentIDs)
		if err != nil {
			return nil, "", err
		}

		bundleDigest := sha256Hex(bundleBytes)
		bundle = map[string]interface{}{
			"mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
			"digest":    "sha256:" + bundleDigest,
			"sizeBytes": len(bundleBytes),
			"url":       fmt.Sprintf("/api/v1/clients/%s/bundles/sha256:%s", clientID, bundleDigest),
		}

		for _, deploymentID := range deploymentIDs {
			yamlBytes := buildDeploymentYAML(clientID, deploymentID)
			deploymentDigest := sha256Hex(yamlBytes)
			refs = append(refs, map[string]interface{}{
				"deploymentId": deploymentID,
				"digest":       "sha256:" + deploymentDigest,
				"sizeBytes":    len(yamlBytes),
				"url":          fmt.Sprintf("/api/v1/clients/%s/deployments/%s/sha256:%s", clientID, deploymentID, deploymentDigest),
			})
		}
	}

	manifest := map[string]interface{}{
		"manifestVersion": 1,
		"bundle":          bundle,
		"deployments":     refs,
	}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}

	return manifest, sha256Hex(manifestBytes), nil
}

func acceptsManifest(headerValue string) bool {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return true
	}

	return strings.Contains(headerValue, "application/vnd.margo.manifest.v1+json") ||
		strings.Contains(headerValue, "*/*")
}

// ===== HTTP HANDLERS =====

// GET /health
func handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

// GET /api/v1/discovery
func handleDiscovery(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]interface{}{
		"name":     "Mock WFM Server for Device Supplier",
		"version":  "1.0.0",
		"persona":  "device_supplier",
		"spec_url": "https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml",
		"endpoints": []string{
			"GET  /api/v1/onboarding/certificate",
			"POST /api/v1/onboarding",
			"POST /api/v1/clients/{clientId}/capabilities",
			"PUT  /api/v1/clients/{clientId}/capabilities",
			"GET  /api/v1/clients/{clientId}/deployments",
			"GET  /api/v1/clients/{clientId}/bundles/{digest}",
			"GET  /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}",
			"POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status",
		},
	})
}

func handleGetCertificate(w http.ResponseWriter, r *http.Request) {
	// Load and return the actual CA certificate from disk
	caCertPath := "./certs/ca-cert.pem"
	caCertBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Printf("⚠ Failed to read CA certificate from %s: %v", caCertPath, err)
		respondJSON(w, 500, ResponseError{Error: "Failed to read CA certificate"})
		return
	}
	
	// Return the PEM certificate as string
	respondJSON(w, 200, map[string]string{
		"certificate": string(caCertBytes),
	})
}


// POST /api/v1/onboarding - Device onboarding (validates using assertions.json)
// NOTE: Onboarding does NOT require signature verification - device is unknown at this point
func handleOnboarding(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Onboarding] 📨 Request received from %s", r.RemoteAddr)
	
	// Read body first (needed for validation)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	// Parse body early so we can extract the client cert for validation
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid JSON body"})
		return
	}

	// For onboarding, the client's cert is in the request body (NO SIGNATURE VERIFICATION)
	certRaw, _ := body["certificate"].(string)
	if certRaw == "" {
		respondJSON(w, 400, ResponseError{Error: "certificate field is required"})
		return
	}

	// Optionally accept the WFM root CA certificate from the client during onboarding
	caCertRaw, _ := body["caCertificate"].(string)
	if caCertRaw != "" {
		serverCACert, err := loadServerCACertificate()
		if err != nil {
			log.Printf("[Onboarding] failed to read server root CA certificate: %v", err)
			respondJSON(w, 500, ResponseError{Error: "internal server error"})
			return
		}
		if normalizeCertificateString(caCertRaw) != normalizeCertificateString(serverCACert) {
			respondJSON(w, 400, ResponseError{Error: "caCertificate does not match server root CA certificate"})
			return
		}
	}

	// NOTE: Per Margo spec and Eclipse Symphony implementation, signature verification
	// is NOT performed on onboarding. Onboarding only validates the request structure
	// and certificate state. Signature verification is enforced on authenticated endpoints
	// (capabilities, deployments, status) after device registration.

	// VALIDATE USING ASSERTIONS FROM JSON
	errors := validateRequest("POST_onboarding", body)
	if len(errors) > 0 {
		statusCode, payload := validationErrorResponse("POST_onboarding", errors)
		respondJSON(w, statusCode, payload)
		return
	}

	if isRejectedCertificate(certRaw) {
		respondJSON(w, 403, ResponseError{Error: "Client rejected"})
		return
	}

	// Create new client
	mu.Lock()
	clientID := uuid.New().String()
	clients[clientID] = ClientData{
		ID:              clientID,
		Certificate:     certRaw,
		OnboardedAt:     time.Now().UTC(),
		DeploymentsData: []string{defaultDeploymentID},
	}
	ensureDefaultDeployment(clientID)
	mu.Unlock()

	// Persist to disk
	if err := saveClientsToFile(); err != nil {
		log.Printf("[Onboarding] ⚠ Failed to persist client: %v", err)
	}
	if err := saveDeploymentsToFile(); err != nil {
		log.Printf("[Onboarding] ⚠ Failed to persist deployments: %v", err)
	}

	log.Printf("[Onboarding] ✅ Device accepted: %s", clientID)

	respondJSON(w, 201, map[string]interface{}{"clientId": clientID})
}

// POST /api/v1/clients/{clientId}/capabilities - Validates using assertions.json
func handlePostCapabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientId"]

	log.Printf("[Capabilities] 📨 Request received for client: %s from %s", clientID, r.RemoteAddr)

	// Validate client exists and retrieve cert for signature verification
	mu.RLock()
	client, exists := clients[clientID]
	mu.RUnlock()

	if !exists {
		log.Printf("[Capabilities] ⚠ Client not found: %s", clientID)
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Client not found: %s", clientID)})
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	// RFC 9421: verify Content-Digest and signature
	if err := verifyRFC9421Signature(r, client.Certificate, bodyBytes); err != nil {
		log.Printf("[Capabilities] Signature verification failed for %s: %v", clientID, err)
		if strings.Contains(err.Error(), "Content-Digest") {
			respondJSON(w, 400, ResponseError{Error: "Missing or invalid content-digest header"})
			return
		}
		respondJSON(w, 401, signatureAuthError(err.Error()))
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid JSON body"})
		return
	}

	// VALIDATE USING ASSERTIONS FROM JSON
	errors := validateRequest("POST_capabilities", body)
	if len(errors) > 0 {
		statusCode, payload := validationErrorResponse("POST_capabilities", errors)
		respondJSON(w, statusCode, payload)
		return
	}

	// Store capabilities
	mu.Lock()
	client = clients[clientID]
	client.Capabilities = body
	clients[clientID] = client
	mu.Unlock()

	log.Printf("[Capabilities] Accepted for client: %s", clientID)

	respondJSON(w, 201, map[string]string{"status": "capabilities_received"})
}

// PUT /api/v1/clients/{clientId}/capabilities
func handlePutCapabilities(w http.ResponseWriter, r *http.Request) {
	// Same validation as POST
	handlePostCapabilities(w, r)
}

// GET /api/v1/clients/{clientId}/deployments
func handleGetDeployments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientId"]

	log.Printf("[Deployments] 📨 GET request received for client: %s from %s", clientID, r.RemoteAddr)

	// Validate client exists and retrieve cert
	mu.RLock()
	client, exists := clients[clientID]
	mu.RUnlock()

	if !exists {
		log.Printf("[Deployments] ⚠ Client not found: %s", clientID)
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Client not found: %s", clientID)})
		return
	}

	// Check Accept header
	log.Printf("[Deployments] Accept header: %s", r.Header.Get("Accept"))

	// RFC 9421 signature verification for GET operations
	if err := verifyRFC9421Signature(r, client.Certificate, nil); err != nil {
		log.Printf("[Deployments] Signature verification failed for %s: %v", clientID, err)
		respondJSON(w, 401, signatureAuthError(err.Error()))
		return
	}

	if !acceptsManifest(r.Header.Get("Accept")) {
		w.WriteHeader(406)
		return
	}

	manifest, etag, err := buildStateManifest(clientID, client.DeploymentsData)
	if err != nil {
		respondJSON(w, 500, ResponseError{Error: "Failed to build deployment manifest"})
		return
	}

	if normalizeETag(r.Header.Get("If-None-Match")) == etag {
		w.Header().Set("ETag", quoteETag(etag))
		w.WriteHeader(304)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.margo.manifest.v1+json")
	w.Header().Set("ETag", quoteETag(etag))
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(manifest)
}

// GET /api/v1/clients/{clientId}/bundles/{digest}
func handleGetBundle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientId"]
	digest := vars["digest"]

	mu.RLock()
	client, exists := clients[clientID]
	mu.RUnlock()

	if !exists {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Client not found: %s", clientID)})
		return
	}

	// RFC 9421 signature verification for GET operations
	if err := verifyRFC9421Signature(r, client.Certificate, nil); err != nil {
		log.Printf("[Bundle] Signature verification failed for %s: %v", clientID, err)
		respondJSON(w, 401, signatureAuthError(err.Error()))
		return
	}

	bundleBytes, err := buildBundleArchive(clientID, client.DeploymentsData)
	if err != nil {
		respondJSON(w, 500, ResponseError{
			Error: "Failed to build deployment bundle",
		})
		return
	}

	expectedDigest := sha256Hex(bundleBytes)
	normalizedDigest := strings.TrimPrefix(digest, "sha256:")
	if normalizedDigest != expectedDigest {
		respondJSON(w, 404, ResponseError{
			Error: fmt.Sprintf("Bundle not found for digest: %s", digest),
		})
		return
	}

	if normalizeETag(r.Header.Get("If-None-Match")) == digest {
		w.Header().Set("ETag", quoteETag(digest))
		w.WriteHeader(304)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.margo.bundle.v1+tar+gzip")
	w.Header().Set("ETag", quoteETag(digest))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(200)
	w.Write(bundleBytes)
}

// GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}
func handleGetDeploymentManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientId"]
	deploymentID := vars["deploymentId"]
	digest := vars["digest"]

	mu.RLock()
	client, exists := clients[clientID]
	mu.RUnlock()

	if !exists {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Client not found: %s", clientID)})
		return
	}

	// RFC 9421 signature verification for GET operations
	if err := verifyRFC9421Signature(r, client.Certificate, nil); err != nil {
		log.Printf("[DeploymentManifest] Signature verification failed for %s: %v", clientID, err)
		respondJSON(w, 401, signatureAuthError(err.Error()))
		return
	}

	found := false
	for _, knownDeploymentID := range client.DeploymentsData {
		if knownDeploymentID == deploymentID {
			found = true
			break
		}
	}
	if !found {
		respondJSON(w, 404, ResponseError{
			Error: fmt.Sprintf("Deployment not found: %s", deploymentID),
		})
		return
	}

	yamlBytes := buildDeploymentYAML(clientID, deploymentID)
	expectedDigest := sha256Hex(yamlBytes)
	normalizedDigest := strings.TrimPrefix(digest, "sha256:")
	if normalizedDigest != expectedDigest {
		respondJSON(w, 404, ResponseError{
			Error: fmt.Sprintf("Deployment not found for digest: %s", digest),
		})
		return
	}

	if normalizeETag(r.Header.Get("If-None-Match")) == digest {
		w.Header().Set("ETag", quoteETag(digest))
		w.WriteHeader(304)
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("ETag", quoteETag(digest))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Vary", "Accept-Encoding")
	w.WriteHeader(200)
	w.Write(yamlBytes)
}

// POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status - Validates using assertions.json
func handlePostStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientId"]
	deploymentID := vars["deploymentId"]

	// Validate client exists and retrieve cert
	mu.RLock()
	client, exists := clients[clientID]
	mu.RUnlock()

	if !exists {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Client not found: %s", clientID)})
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	// RFC 9421: full signature verification using stored client cert
	if err := verifyRFC9421Signature(r, client.Certificate, bodyBytes); err != nil {
		log.Printf("[Status] Signature verification failed for %s: %v", clientID, err)
		if strings.Contains(err.Error(), "Content-Digest") {
			respondJSON(w, 400, ResponseError{Error: "Missing or invalid content-digest header"})
			return
		}
		respondJSON(w, 401, signatureAuthError(err.Error()))
		return
	}

	// Parse body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid JSON body"})
		return
	}

	if bodyDeploymentID, ok := getFieldValue(body, "deploymentId"); ok {
		if bodyDeploymentIDStr, ok := bodyDeploymentID.(string); !ok || bodyDeploymentIDStr != deploymentID {
			respondJSON(w, 422, ResponseError{
				Status: "validation_failed",
				Errors: []ValidationError{
					{RuleID: "status-path-001", Error: "deploymentId in body must match deploymentId in path"},
				},
			})
			return
		}
	}

	// VALIDATE USING ASSERTIONS FROM JSON
	errors := validateRequest("POST_status", body)
	if len(errors) > 0 {
		statusCode, payload := validationErrorResponse("POST_status", errors)
		respondJSON(w, statusCode, payload)
		return
	}

	// Store status update
	mu.Lock()
	deploymentMapKey := deploymentKey(clientID, deploymentID)
	deployment, depExists := deployments[deploymentMapKey]
	if !depExists {
		deployment = DeploymentData{
			ID:       deploymentID,
			ClientID: clientID,
		}
	}
	deployment.StatusHistory = append(deployment.StatusHistory, body)
	deployments[deploymentMapKey] = deployment
	mu.Unlock()

	log.Printf("[Status] Update for deployment %s under client %s", deploymentID, clientID)

	// Response per spec
	respondJSON(w, 200, map[string]string{
		"acknowledgement": "received",
	})
}

// ===== RESPONSE HELPERS =====

func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// ===== TLS CERTIFICATE HELPERS =====

// generateCASignedServerCert generates a server certificate signed by the CA certificate
func generateCASignedServerCert(caCertPath, caKeyPath, serverCertFile, serverKeyFile string) error {
	log.Printf("Generating server certificate signed by CA...")

	// Read CA certificate
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return fmt.Errorf("failed to parse CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Read CA private key
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA private key: %w", err)
	}

	block, _ = pem.Decode(caKeyPEM)
	if block == nil {
		return fmt.Errorf("failed to parse CA private key PEM")
	}

	caKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format
		caKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse CA private key: %w", err)
		}
	}

	// Generate server private key
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate server private key: %w", err)
	}

	// Create server certificate template
	serverCert := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:      []string{"IN"},
			Province:     []string{"GGN"},
			Locality:     []string{"Sector 48"},
			Organization: []string{"Margo"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{"localhost", "127.0.0.1"},
	}

	// Create certificate signed by CA
	certBytes, err := x509.CreateCertificate(rand.Reader, &serverCert, caCert, &serverPrivateKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create server certificate signed by CA: %w", err)
	}

	// Write server certificate to file
	certOut, err := os.Create(serverCertFile)
	if err != nil {
		return fmt.Errorf("failed to open server cert file for writing: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return fmt.Errorf("failed to write server certificate to file: %w", err)
	}

	// Write server private key to file
	keyOut, err := os.Create(serverKeyFile)
	if err != nil {
		return fmt.Errorf("failed to open server key file for writing: %w", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(serverPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal server private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write server private key to file: %w", err)
	}

	log.Printf("✓ Server certificate signed by CA: %s", serverCertFile)
	return nil
}

// ensureTLSCertificates ensures TLS certificates exist, creating them if needed
func ensureTLSCertificates() (certFile, keyFile string, err error) {
	// Define certificate paths
	certDir := "./certs"
	caCertPath := filepath.Join(certDir, "ca-cert.pem")
	caKeyPath := filepath.Join(certDir, "ca-key.pem")
	certFile = filepath.Join(certDir, "server-cert.pem")
	keyFile = filepath.Join(certDir, "server-key.pem")

	// Create certs directory if needed
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create certs directory: %w", err)
	}

	// First, copy CA files from home directory if they don't exist locally
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeCACert := filepath.Join(homeDir, "certs", "ca-cert.pem")
		homeCACKey := filepath.Join(homeDir, "certs", "ca-private.key")

		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			if homeData, err := os.ReadFile(homeCACert); err == nil {
				if err := os.WriteFile(caCertPath, homeData, 0644); err == nil {
					log.Printf("✓ Copied CA certificate from home")
				}
			}
		}

		if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
			if homeData, err := os.ReadFile(homeCACKey); err == nil {
				if err := os.WriteFile(caKeyPath, homeData, 0600); err == nil {
					log.Printf("✓ Copied CA private key from home")
				}
			}
		}
	}

	// Check if server certificates already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			log.Printf("✓ Using existing server TLS certificates")
			return certFile, keyFile, nil
		}
	}

	// Generate server certificate
	_, caKeyErr := os.Stat(caKeyPath)
	_, caCertErr := os.Stat(caCertPath)

	if caCertErr == nil && caKeyErr == nil {
		// CA files exist, use them to sign server cert
		if err := generateCASignedServerCert(caCertPath, caKeyPath, certFile, keyFile); err != nil {
			log.Printf("⚠ Failed to generate CA-signed cert (%v)", err)
			return "", "", err
		}
	} else {
		log.Printf("⚠ CA files not available, server certificates should be pre-generated")
	}

	// Verify files were created
	if _, err := os.Stat(certFile); err != nil {
		return "", "", fmt.Errorf("failed to create server certificate")
	}

	return certFile, keyFile, nil
}

// ===== PERSISTENCE FUNCTIONS =====

func ensureDataDirectory() error {
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	return nil
}

func loadClientsFromFile() error {
	if _, err := os.Stat(ClientsFile); os.IsNotExist(err) {
		log.Printf("ℹ No persisted clients found at %s (first startup)", ClientsFile)
		return nil
	}

	data, err := os.ReadFile(ClientsFile)
	if err != nil {
		return fmt.Errorf("failed to read clients file: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if err := json.Unmarshal(data, &clients); err != nil {
		return fmt.Errorf("failed to unmarshal clients: %w", err)
	}

	log.Printf("✓ Loaded %d clients from persistent storage", len(clients))
	for id := range clients {
		log.Printf("  - Client: %s", id)
	}
	return nil
}

func saveClientsToFile() error {
	mu.RLock()
	defer mu.RUnlock()

	log.Printf("[Persistence] Saving %d clients to disk...", len(clients))

	data, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal clients: %w", err)
	}

	if err := os.WriteFile(ClientsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write clients file: %w", err)
	}

	log.Printf("[Persistence] ✅ Clients saved to %s", ClientsFile)
	return nil
}

func loadDeploymentsFromFile() error {
	if _, err := os.Stat(DeploymentsFile); os.IsNotExist(err) {
		log.Printf("ℹ No persisted deployments found at %s (first startup)", DeploymentsFile)
		return nil
	}

	data, err := os.ReadFile(DeploymentsFile)
	if err != nil {
		return fmt.Errorf("failed to read deployments file: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if err := json.Unmarshal(data, &deployments); err != nil {
		return fmt.Errorf("failed to unmarshal deployments: %w", err)
	}

	log.Printf("Loaded %d deployments from persistent storage", len(deployments))
	return nil
}

func saveDeploymentsToFile() error {
	mu.RLock()
	defer mu.RUnlock()

	data, err := json.MarshalIndent(deployments, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployments: %w", err)
	}

	if err := os.WriteFile(DeploymentsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write deployments file: %w", err)
	}

	return nil
}

// ===== MAIN =====

func main() {
	// Ensure data directory exists
	if err := ensureDataDirectory(); err != nil {
		log.Fatal(err)
	}

	// Always reset persisted data on startup — test clients from previous runs
	// are invalid (certs change each run) and only pollute the state.
	// Set KEEP_DATA=true to skip this and resume from previous state.
	if os.Getenv("KEEP_DATA") != "true" {
		log.Printf("🧹 Clearing persisted data for a clean test run (set KEEP_DATA=true to skip)")
		os.Remove(ClientsFile)
		os.Remove(DeploymentsFile)
	}

	// Load persisted data from previous runs (if not cleaned)
	if err := loadClientsFromFile(); err != nil {
		log.Printf("⚠ Failed to load clients: %v", err)
	}
	if err := loadDeploymentsFromFile(); err != nil {
		log.Printf("⚠ Failed to load deployments: %v", err)
	}

	// Load assertions from JSON (DATA-DRIVEN)
	var err error
	assertionsConfig, err = loadAssertions("manifests/assertions.json")
	if err != nil {
		log.Fatal(err)
	}

	// Setup routes
	router := mux.NewRouter()

	// Add logging middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[Router] %s %s from %s", r.Method, r.RequestURI, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	})

	// Health check
	router.HandleFunc("/health", handleHealth).Methods("GET")

	// Discovery
	router.HandleFunc("/api/v1/discovery", handleDiscovery).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/discovery", handleDiscovery).Methods("GET")

	router.HandleFunc("/v1alpha2/margo/api/v1/onboarding/certificate", handleGetCertificate).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/onboarding", handleOnboarding).Methods("POST")

	// Capabilities
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/capabilities", handlePostCapabilities).Methods("POST")
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/capabilities", handlePutCapabilities).Methods("PUT")

	// Deployments
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/deployments", handleGetDeployments).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/bundles/{digest}", handleGetBundle).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}", handleGetDeploymentManifest).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/clients/{clientId}/deployments/{deploymentId}/status", handlePostStatus).Methods("POST")

	// Ensure TLS certificates are available
	certFile, keyFile, err := ensureTLSCertificates()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("🚀 Mock WFM Server starting on https://localhost%s", WFMPort)
	if err := http.ListenAndServeTLS(WFMPort, certFile, keyFile, router); err != nil {
		log.Fatal(err)
	}
}
