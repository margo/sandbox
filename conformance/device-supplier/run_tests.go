package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lestrrat-go/htmsig/component"
	htmsighttp "github.com/lestrrat-go/htmsig/http"
)

const (
	certDir = "./certs"
)

// WFMServer is the mock WFM server base URL; defaults below, overridable via -url.
var WFMServer = "https://localhost:3001/v1alpha2/margo"

// ClaimedAppVersion is the artifact/app version under test, set via -claimed-app-version.
var ClaimedAppVersion = "unknown"

// CTTMargoVersion is the Margo spec version this conformance tool validates against, set via -ctt-margo-version.
var CTTMargoVersion = "unknown"

// verbose gates printing the full JSON response body for every step; set from -verbose in main().
var verbose bool

// tlsSkipClient returns an HTTP client that skips TLS verification.
// Required because the mock-server uses a self-signed certificate.
func tlsSkipClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in test env
		},
	}
}

// Test structures (data-driven)
type TestScenario struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	FixedFirst  bool       `json:"fixed_first,omitempty"`
	Steps       []TestStep `json:"steps"`
}

type TestStep struct {
	ID                        string                 `json:"id"`
	Name                      string                 `json:"name"`
	Method                    string                 `json:"method"`
	Endpoint                  string                 `json:"endpoint"`
	RequestBody               map[string]interface{} `json:"request_body,omitempty"`
	Headers                   map[string]string      `json:"headers,omitempty"`
	SkipSigning               bool                   `json:"skip_signing,omitempty"`
	SkipCertificateInjection  bool                   `json:"skip_certificate_injection,omitempty"`
	ExpectedStatus            int                    `json:"expected_status"`
	Validations               []StepValidation       `json:"validations"`
	ExtractContext            map[string]string      `json:"extract_context,omitempty"`
}

type StepValidation struct {
	Field     string      `json:"field"`
	Operation string      `json:"operation"`
	Value     interface{} `json:"value,omitempty"`
}

type TestResult struct {
	ScenarioID   string      `json:"scenario_id"`
	ScenarioName string      `json:"scenario_name"`
	StepID       string      `json:"step_id"`
	StepName     string      `json:"step_name"`
	Status       string      `json:"status"` // "pass", "fail"
	Reason       string      `json:"reason,omitempty"`
	StatusCode   int         `json:"status_code"`
	Response     interface{} `json:"response,omitempty"`
	Timestamp    string      `json:"timestamp"`
}

// Test runner context (stores data between steps)
type TestContext struct {
	ClientID     string
	Capabilities map[string]interface{}
	Deployments  []string
	Data         map[string]interface{}
}

// ===== MAIN TEST RUNNER =====

func main() {
	// CLI flags for filtering
	urlFlag := flag.String("url", WFMServer, "Mock WFM Server base URL (e.g. https://192.168.1.10:3001/v1alpha2/margo)")
	scenarioFilter := flag.String("scenario", "", "Run only the scenario with this ID (e.g. scenario-onboarding)")
	stepFilter := flag.String("step", "", "Run only the step with this ID within the matched scenario (e.g. step-1.2)")
	scenariosFile := flag.String("file", "device-scenarios/test-scenarios.json", "Path to test scenarios JSON file")
	flexibleOrder := flag.Bool("flexible-order", false, "Run the one scenario marked \"fixed_first\" first, then run all remaining scenarios in a random relative order (proves the mock server doesn't require a fixed call sequence). Opt-in; default behavior is unchanged.")
	seedFlag := flag.Int64("seed", 0, "Random seed for -flexible-order shuffling (0 = derive from current time; the seed actually used is always printed for reproducibility)")
	clientIDFlag := flag.String("client-id", "", "Pre-existing clientId to seed {clientId} with instead of onboarding fresh. Lets you run scenario files one at a time by hand: run onboarding.json first, copy the clientId it prints, then pass it here for subsequent files.")
	verboseFlag := flag.Bool("verbose", false, "Print the full JSON response body (plus response headers) for every step. Useful when running one scenario file at a time by hand to inspect exactly what the server returned.")
	claimedAppVersionFlag := flag.String("claimed-app-version", "unknown", "Claimed App Version — the artifact/app version under test, from the selected group's group.json")
	cttMargoVersionFlag := flag.String("ctt-margo-version", "1.0.0-rc.2", "CTT Margo Version — the Margo spec version this conformance tool validates against")
	flag.Parse()
	WFMServer = *urlFlag
	verbose = *verboseFlag
	ClaimedAppVersion = *claimedAppVersionFlag
	CTTMargoVersion = *cttMargoVersionFlag

	if err := ensureCertificates(); err != nil {
		log.Fatalf("Error preparing certificates: %v", err)
	}

	// Load test scenarios from JSON file
	scenarios, err := loadScenarios(*scenariosFile)
	if err != nil {
		log.Fatalf("Error loading test scenarios: %v", err)
	}

	if len(scenarios) == 0 {
		log.Fatal("No test scenarios found")
	}

	fmt.Println(`
╔══════════════════════════════════════════════════════════════════════════════╗
║              Device Supplier Conformance Test Runner                         ║
║                   Data-Driven Test Framework                                 ║
║                                                                              ║
║  Testing against: ` + WFMServer + `                                 ║
║  Spec: Margo Management Interface API 1.0.0-rc.2                            ║
╚══════════════════════════════════════════════════════════════════════════════╝
	`)
	fmt.Printf("Claimed App Version: %s  ·  CTT Margo Version: %s\n", ClaimedAppVersion, CTTMargoVersion)
	if ClaimedAppVersion != "unknown" && ClaimedAppVersion != CTTMargoVersion {
		fmt.Printf("\033[32m⚠ Version Mismatch: Claimed App Version (%s) differs from CTT Margo Version (%s)\033[0m\n", ClaimedAppVersion, CTTMargoVersion)
	}

	// Wait for server to be ready
	if !waitForServer(5 * time.Second) {
		log.Fatalf("❌ WFM Server not responding at %s", WFMServer)
	}
	fmt.Println("✅ WFM Server is ready")
	fmt.Println()

	// Run all test scenarios
	var allResults []TestResult
	passCount := 0
	failCount := 0

	if *flexibleOrder {
		allResults, passCount, failCount = runFlexibleOrder(scenarios, *scenarioFilter, *stepFilter, *seedFlag)
	} else {
		for _, scenario := range scenarios {
			// Apply scenario filter
			if *scenarioFilter != "" && scenario.ID != *scenarioFilter {
				continue
			}

			ctx := &TestContext{
				ClientID: *clientIDFlag,
				Data:     make(map[string]interface{}),
			}

			results, p, f := runScenarioSteps(scenario, ctx, *stepFilter)
			allResults = append(allResults, results...)
			passCount += p
			failCount += f
		}
	}

	// Print summary
	fmt.Println(`╔══════════════════════════════════════════════════════════════════════════════╗`)
	fmt.Printf("║  Test Results: %d PASSED, %d FAILED (Total: %d)\n", passCount, failCount, passCount+failCount)
	fmt.Println(`╚══════════════════════════════════════════════════════════════════════════════╝`)

	// Save results to file
	saveResults(allResults)

	if failCount > 0 {
		os.Exit(1)
	}
}

// runScenarioSteps runs every step of a single scenario against ctx, printing
// progress and accumulating pass/fail counts. Shared by the default sequential
// runner and runFlexibleOrder so both paths execute steps identically.
func runScenarioSteps(scenario TestScenario, ctx *TestContext, stepFilter string) ([]TestResult, int, int) {
	fmt.Printf("▶ Running Scenario: %s (%s)\n", scenario.Name, scenario.ID)
	fmt.Printf("  Description: %s\n", scenario.Description)

	var results []TestResult
	pass, fail := 0, 0

	for _, step := range scenario.Steps {
		// Apply step filter
		if stepFilter != "" && step.ID != stepFilter {
			continue
		}

		fmt.Printf("  → Step: %s\n", step.Name)

		result := executeStep(step, ctx)
		result.ScenarioID = scenario.ID
		result.ScenarioName = scenario.Name
		results = append(results, result)

		if verbose {
			if respJSON, err := json.MarshalIndent(result.Response, "    ", "  "); err == nil {
				fmt.Printf("    ↳ HTTP %d response:\n    %s\n", result.StatusCode, respJSON)
			}
		}

		if result.Status == "pass" {
			fmt.Printf("    ✅ PASS - HTTP %d (Expected: %d)\n", result.StatusCode, step.ExpectedStatus)
			pass++
		} else {
			fmt.Printf("    ❌ FAIL - %s\n", result.Reason)
			fail++
		}
	}

	fmt.Println()
	return results, pass, fail
}

// runFlexibleOrder runs the one scenario marked "fixed_first" (if any) first,
// capturing its clientId into a context shared by every other scenario, then
// runs the remaining scenarios in a random relative order. Only ClientID is
// shared globally — each scenario still gets its own empty Data map, so any
// scenario-local extractions (deploymentId, digest, etc.) stay scenario-local
// and self-contained regardless of run order.
func runFlexibleOrder(scenarios []TestScenario, scenarioFilter, stepFilter string, seed int64) ([]TestResult, int, int) {
	var filtered []TestScenario
	for _, s := range scenarios {
		if scenarioFilter != "" && s.ID != scenarioFilter {
			continue
		}
		filtered = append(filtered, s)
	}

	var first []TestScenario
	var rest []TestScenario
	for _, s := range filtered {
		if s.FixedFirst {
			first = append(first, s)
		} else {
			rest = append(rest, s)
		}
	}
	if len(first) > 1 {
		log.Fatalf("❌ -flexible-order requires at most one scenario marked \"fixed_first\": true, found %d", len(first))
	}

	var allResults []TestResult
	passCount, failCount := 0, 0
	var sharedClientID string

	if len(first) == 1 {
		fmt.Println("🔒 Fixed-first phase (runs before any shuffled scenario):")
		ctx := &TestContext{Data: make(map[string]interface{})}
		results, p, f := runScenarioSteps(first[0], ctx, stepFilter)
		allResults = append(allResults, results...)
		passCount += p
		failCount += f
		sharedClientID = ctx.ClientID
	} else {
		fmt.Println("⚠ No scenario marked fixed_first found in the filtered set — {clientId} will be empty in shuffled scenarios.")
	}

	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })

	order := make([]string, len(rest))
	for i, s := range rest {
		order[i] = s.ID
	}
	fmt.Printf("🔀 Randomized order (seed=%d): %v\n\n", seed, order)

	for _, scenario := range rest {
		ctx := &TestContext{ClientID: sharedClientID, Data: make(map[string]interface{})}
		results, p, f := runScenarioSteps(scenario, ctx, stepFilter)
		allResults = append(allResults, results...)
		passCount += p
		failCount += f
	}

	return allResults, passCount, failCount
}

// ===== TEST EXECUTION =====

func executeStep(step TestStep, ctx *TestContext) TestResult {
	result := TestResult{
		StepID:     step.ID,
		StepName:   step.Name,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		StatusCode: 0,
	}

	// Prepare endpoint with context interpolation
	endpoint := interpolateContext(step.Endpoint, ctx)

	// Prepare request body
	var bodyReader io.Reader
	var bodyBytes []byte
	if step.RequestBody != nil {
		body := interpolateContextInObject(step.RequestBody, ctx)
		
		// Resolve cert path values (e.g. ./certs/device-cert.pem) to PEM content.
		// Negative tests can opt out via skip_certificate_injection to keep literal strings.
		if certRaw, hasCert := body["certificate"]; hasCert && !step.SkipCertificateInjection {
			if certPath, ok := certRaw.(string); ok {
				resolvedCert, certErr := resolveCertificateValue(certPath)
				if certErr != nil {
					result.Status = "fail"
					result.Reason = certErr.Error()
					return result
				}
				body["certificate"] = resolvedCert
			}
		}
		
		bodyBytes, _ = json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
		result.Response = body
	}

	// Create HTTP request
	req, err := http.NewRequest(step.Method, WFMServer+endpoint, bodyReader)
	if err != nil {
		result.Status = "fail"
		result.Reason = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")

	// RFC 9421: sign all requests (adds Signature-Input, Signature, Content-Digest)
	// unless skip_signing is true
	if !step.SkipSigning {
		if err := signRequest(req, bodyBytes); err != nil {
			result.Status = "fail"
			result.Reason = fmt.Sprintf("Failed to sign request: %v", err)
			return result
		}
	}

	// Add custom headers from test definition (after signing so they can override if needed)
	for key, value := range step.Headers {
		req.Header.Set(key, interpolateHeaderValue(value, ctx))
	}

	// Execute request using TLS-skip client (self-signed cert on mock-server)
	client := tlsSkipClient()
	resp, err := client.Do(req)
	if err != nil {
		result.Status = "fail"
		result.Reason = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// Read response body
	respBody, _ := io.ReadAll(resp.Body)
	var respData interface{}
	json.Unmarshal(respBody, &respData)
	headers := make(map[string]interface{})
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	if dataMap, ok := respData.(map[string]interface{}); ok {
		dataMap["_headers"] = headers
		result.Response = dataMap
		respData = dataMap
	} else {
		result.Response = map[string]interface{}{
			"_headers": headers,
			"_raw":     string(respBody),
		}
		respData = result.Response
	}

	// Validate status code
	if resp.StatusCode != step.ExpectedStatus {
		result.Status = "fail"
		result.Reason = fmt.Sprintf("Expected HTTP %d, got %d", step.ExpectedStatus, resp.StatusCode)
		return result
	}

	// Run validations
	for _, validation := range step.Validations {
		if !validateResponse(respData, validation, ctx) {
			result.Status = "fail"
			result.Reason = fmt.Sprintf("Validation failed for field '%s': %s", validation.Field, validation.Operation)
			return result
		}
	}

	// Extract context for next steps
	if len(step.ExtractContext) > 0 {
		for varName, jsonPath := range step.ExtractContext {
			value := extractJSONPath(respData, jsonPath)
			if value != nil {
				ctx.Data[varName] = value
				fmt.Printf("    📎 Captured %s = %v\n", varName, value)
				if varName == "clientId" {
					ctx.ClientID = value.(string)
				}
			}
		}
	}

	result.Status = "pass"
	return result
}

func resolveCertificateValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value, nil
	}

	// Treat path-like values as cert files to be loaded.
	if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "certs/") {
		cleanPath := filepath.Clean(trimmed)
		certData, err := os.ReadFile(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to load certificate from %s: %w", cleanPath, err)
		}
		// log.Printf("[cert] Loaded certificate from %s (%d bytes)", cleanPath, len(certData))
		return string(certData), nil
	}

	return value, nil
}

func ensureCertificates() error {
	requiredFiles := []string{
		"ca-cert.pem",
		"ca-key.pem",
		"server-cert.pem",
		"server-key.pem",
		"device-key.pem",
		"device-cert.pem",
	}

	for _, fileName := range requiredFiles {
		if _, err := os.Stat(filepath.Join(certDir, fileName)); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("🔐 Required certs missing, generating them with generate-certs.sh...")
				cmd := exec.Command("bash", "generate-certs.sh", certDir, "localhost")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if runErr := cmd.Run(); runErr != nil {
					return fmt.Errorf("generate-certs.sh failed: %w", runErr)
				}
				return nil
			}
			return fmt.Errorf("failed to inspect %s: %w", filepath.Join(certDir, fileName), err)
		}
	}

	return nil
}

func buildContentDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// ===== RFC 9421 CLIENT-SIDE SIGNING =====

// defaultDeviceKeyPath is the private key used to sign requests.
// It matches ./certs/device-cert.pem generated by generate-certs.sh.
const defaultDeviceKeyPath = "./certs/device-key.pem"

func getDeviceKeyPath() string {
	if customPath := strings.TrimSpace(os.Getenv("DEVICE_PRIVATE_KEY_PATH")); customPath != "" {
		return customPath
	}
	return defaultDeviceKeyPath
}

// loadDevicePrivateKey loads the PEM private key from deviceKeyPath.
func loadDevicePrivateKey() (interface{}, error) {
	deviceKeyPath := getDeviceKeyPath()
	data, err := os.ReadFile(deviceKeyPath)
	if err != nil {
		return nil, fmt.Errorf("device private key not found at %s: %w", deviceKeyPath, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to PEM-decode device private key")
	}
	// Try PKCS8 first (RSA or ECDSA wrapped)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// Fall back to PKCS1 RSA
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// Try EC key
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("unrecognized private key format")
}

// signRequest adds RFC 9421 Signature-Input, Signature, and Content-Digest headers
// using the htmsig library — the same library used by the server for verification.
func signRequest(req *http.Request, bodyBytes []byte) error {
	key, err := loadDevicePrivateKey()
	if err != nil {
		return fmt.Errorf("could not load device key: %w", err)
	}

	// log.Printf("[sign] Request: %s %s, body length: %d bytes", req.Method, req.URL.Path, len(bodyBytes))

	// Build Content-Digest header for requests with a body
	comps := []component.Identifier{
		component.Method(),
		component.TargetURI(),
	}
	if len(bodyBytes) > 0 {
		digest := buildContentDigest(bodyBytes)
		// log.Printf("[sign] Content-Digest computed: %s (body first 100 chars: %.100s)", digest, string(bodyBytes))
		req.Header.Set("Content-Digest", digest)
		comps = append(comps, component.New("content-digest"))
	} else {
		// log.Printf("[sign] No body - Content-Digest not set")
	}

	signer := htmsighttp.NewSigner(key, "device-key",
		htmsighttp.WithComponents(comps...),
		htmsighttp.WithLabel("sig1"),
	)
	if err := signer.SignRequest(context.Background(), req); err != nil {
		return fmt.Errorf("htmsig SignRequest failed: %w", err)
	}
	return nil
}

// ===== UTILITIES =====

func loadScenarios(filePath string) ([]TestScenario, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var scenarios []TestScenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		return nil, err
	}

	return scenarios, nil
}

func waitForServer(timeout time.Duration) bool {
	client := tlsSkipClient()
	// Health endpoint is at the root, not under /v1alpha2/margo — derive
	// scheme+host from WFMServer (set via -url) rather than hardcoding
	// localhost, so this actually checks the server under test.
	healthURL := "https://localhost:3001/health"
	if parsed, err := url.Parse(WFMServer); err == nil && parsed.Host != "" {
		healthURL = parsed.Scheme + "://" + parsed.Host + "/health"
	}
	deadline := time.Now().Add(timeout)
	for {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func interpolateContext(endpoint string, ctx *TestContext) string {
	result := endpoint
	result = strings.ReplaceAll(result, "{clientId}", ctx.ClientID)
	for key, value := range ctx.Data {
		result = strings.ReplaceAll(result, "{"+key+"}", fmt.Sprintf("%v", value))
	}
	return result
}

func interpolateContextInObject(obj map[string]interface{}, ctx *TestContext) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range obj {
		result[key] = interpolateValue(value, ctx)
	}
	return result
}

func interpolateValue(value interface{}, ctx *TestContext) interface{} {
	switch typed := value.(type) {
	case string:
		result := strings.ReplaceAll(typed, "{clientId}", ctx.ClientID)
		for ctxKey, ctxVal := range ctx.Data {
			result = strings.ReplaceAll(result, "{"+ctxKey+"}", fmt.Sprintf("%v", ctxVal))
		}
		return result
	case map[string]interface{}:
		return interpolateContextInObject(typed, ctx)
	case []interface{}:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = interpolateValue(item, ctx)
		}
		return result
	default:
		return value
	}
}

func extractJSONPath(data interface{}, path string) interface{} {
	current := data
	for _, part := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]interface{}:
			val, exists := typed[part]
			if !exists {
				for existingKey, existingValue := range typed {
					if strings.EqualFold(existingKey, part) {
						val = existingValue
						exists = true
						break
					}
				}
				if !exists {
					return nil
				}
			}
			current = val
		case []interface{}:
			index := -1
			if _, err := fmt.Sscanf(part, "%d", &index); err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
	}
	return current
}

func interpolateHeaderValue(value string, ctx *TestContext) string {
	return interpolateValue(value, ctx).(string)
}

func validateResponse(data interface{}, validation StepValidation, ctx *TestContext) bool {
	value := extractJSONPath(data, validation.Field)
	if value == nil {
		return false
	}

	expected := validation.Value
	if strValue, ok := validation.Value.(string); ok {
		expected = interpolateHeaderValue(strValue, ctx)
	}

	switch validation.Operation {
	case "equals":
		return value == expected
	case "exists":
		return value != nil
	case "not_empty":
		if str, ok := value.(string); ok {
			return str != ""
		}
		return value != nil
	case "is_string":
		_, ok := value.(string)
		return ok
	case "is_number":
		_, ok := value.(float64)
		return ok
	case "is_array":
		_, ok := value.([]interface{})
		return ok
	case "is_object":
		_, ok := value.(map[string]interface{})
		return ok
	case "contains":
		if str, ok := value.(string); ok {
			expectedStr, ok := expected.(string)
			if !ok {
				return false
			}
			return strings.Contains(str, expectedStr)
		}
		return false
	default:
		return true
	}
}

func saveResults(results []TestResult) {
	// Group by scenario
	scenarios := make(map[string][]TestResult)
	for _, result := range results {
		scenarios[result.ScenarioID] = append(scenarios[result.ScenarioID], result)
	}

	if err := os.MkdirAll("reports", 0755); err != nil {
		fmt.Printf("⚠ Could not create reports directory: %v\n", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05-000Z07:00")
	filename := fmt.Sprintf("reports/conformance-report-%s.html", timestamp)

	report := generateHTMLReport(results)

	if err := os.WriteFile(filename, []byte(report), 0644); err != nil {
		fmt.Printf("⚠ Could not save report: %v\n", err)
		return
	}
	fmt.Printf("📊 Test report saved: %s\n", filename)
}

func generateHTMLReport(results []TestResult) string {
	passCount := 0
	failCount := 0

	for _, r := range results {
		if r.Status == "pass" {
			passCount++
		} else {
			failCount++
		}
	}

	versionWarning := ""
	if ClaimedAppVersion != "unknown" && ClaimedAppVersion != CTTMargoVersion {
		versionWarning = "<div class=\"version-warning\">⚠ Version Mismatch: Claimed App Version (" + ClaimedAppVersion + ") differs from CTT Margo Version (" + CTTMargoVersion + ")</div>\n    "
	}

	html := "<!DOCTYPE html>\n<html>\n<head>\n    <title>Device Supplier Conformance Report</title>\n    <style>\n        body { font-family: Arial, sans-serif; margin: 20px; }\n        .header { background: #333; color: white; padding: 20px; border-radius: 5px; }\n        .summary { margin: 20px 0; }\n        .pass { color: green; font-weight: bold; }\n        .fail { color: red; font-weight: bold; }\n        table { width: 100%; border-collapse: collapse; margin-top: 20px; }\n        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }\n        th { background: #f2f2f2; }\n        .version-warning { margin: 16px 0; padding: 10px 14px; border-radius: 4px; background: #dcfce7; color: #166534; border: 1px solid #86efac; font-size: 13px; font-weight: bold; }\n    </style>\n</head>\n<body>\n    <div class=\"header\">\n        <h1>Device Supplier Conformance Test Report</h1>\n        <p>CTT Margo Version: " + CTTMargoVersion + "</p>\n        <p>Claimed App Version: " + ClaimedAppVersion + "</p>\n        <p>Generated: " + time.Now().Format(time.RFC3339) + "</p>\n    </div>\n    " + versionWarning + "<div class=\"summary\">\n        <h2>Summary</h2>\n        <p>Total Tests: " + fmt.Sprintf("%d", len(results)) + " | <span class=\"pass\">&#x2705; Passed: " + fmt.Sprintf("%d", passCount) + "</span> | <span class=\"fail\">&#x274C; Failed: " + fmt.Sprintf("%d", failCount) + "</span></p>\n        <p>Success Rate: " + fmt.Sprintf("%.1f", float64(passCount)/float64(len(results))*100) + "%</p>\n    </div>\n    <table>\n        <tr><th>Step</th><th>Status</th><th>HTTP Code</th><th>Details</th></tr>\n"

	for _, r := range results {
		statusClass := "pass"
		statusText := "✅ PASS"
		if r.Status == "fail" {
			statusClass = "fail"
			statusText = "❌ FAIL"
		}

		html += fmt.Sprintf(`
        <tr>
            <td>%s</td>
            <td><span class="%s">%s</span></td>
            <td>%d</td>
            <td>%s</td>
        </tr>
`, r.StepName, statusClass, statusText, r.StatusCode, r.Reason)
	}

	html += `
    </table>
</body>
</html>
`

	return html
}
