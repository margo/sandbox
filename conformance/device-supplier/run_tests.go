package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
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
	"sort"
	"strings"
	"time"
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
	// CRIds lists the Margo conformance requirement IDs this scenario exercises;
	// surfaced in the HTML report's coverage summary. A step may add its own.
	CRIds       []string   `json:"crIds,omitempty"`
	FixedFirst  bool       `json:"fixed_first,omitempty"`
	// SigningKey / SigningAlgorithm apply to every step in the scenario unless a
	// step overrides them. Used to exercise the non-default RFC 9421 signature
	// algorithms (MI-012 ecdsa-p384-sha384, MI-014 rsa-v1_5-sha256).
	SigningKey       string     `json:"signing_key,omitempty"`
	SigningAlgorithm string     `json:"signing_algorithm,omitempty"`
	Steps            []TestStep `json:"steps"`
}

type TestStep struct {
	ID                        string                 `json:"id"`
	Name                      string                 `json:"name"`
	CRIds                     []string               `json:"crIds,omitempty"`
	Method                    string                 `json:"method"`
	Endpoint                  string                 `json:"endpoint"`
	RequestBody               map[string]interface{} `json:"request_body,omitempty"`
	Headers                   map[string]string      `json:"headers,omitempty"`
	SkipSigning               bool                   `json:"skip_signing,omitempty"`
	SkipCertificateInjection  bool                   `json:"skip_certificate_injection,omitempty"`
	ExpectedStatus            int                    `json:"expected_status"`
	Validations               []StepValidation       `json:"validations"`
	ExtractContext            map[string]string      `json:"extract_context,omitempty"`
	// ExpectManifestRejected / ExpectManifestAccepted assert the verdict a
	// conformant device client MUST reach on the desired-state manifest in this
	// step's response (see evaluateManifest). "Rejected" passes when the
	// manifest violates a rule a client MUST enforce (bad digest algorithm,
	// digest that doesn't match the artifact, non-increasing manifestVersion);
	// "Accepted" passes when it does not — used to prove a client MUST proceed
	// despite a difference the spec says is not an integrity signal (sizeBytes).
	ExpectManifestRejected    bool                   `json:"expect_manifest_rejected,omitempty"`
	ExpectManifestAccepted    bool                   `json:"expect_manifest_accepted,omitempty"`
	// SigningKey / SigningAlgorithm override the scenario-level signing config
	// for this one step (see TestScenario). MI-012 / MI-014.
	SigningKey       string `json:"signing_key,omitempty"`
	SigningAlgorithm string `json:"signing_algorithm,omitempty"`
	// VerifyTLS makes this step verify the server's TLS certificate against the
	// fetched root CA (certs/ca-cert.pem) instead of skipping verification.
	// ExpectTransportError passes the step when the request fails at the
	// transport layer (e.g. the server cert doesn't chain to the trusted CA).
	// Together they cover MI-018.
	VerifyTLS            bool `json:"verify_tls,omitempty"`
	ExpectTransportError bool `json:"expect_transport_error,omitempty"`
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
	CRIds        []string    `json:"crIds,omitempty"`
	Method       string      `json:"method,omitempty"`
	Endpoint     string      `json:"endpoint,omitempty"`
	Expected     int         `json:"expected_status,omitempty"`
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
	covered, attempted := crIDCoverage(allResults)
	fmt.Println(`╔══════════════════════════════════════════════════════════════════════════════╗`)
	fmt.Printf("║  Test Results: %d PASSED, %d FAILED (Total: %d)\n", passCount, failCount, passCount+failCount)
	fmt.Printf("║  Requirements verified: %d  (referenced by tests: %d)\n", len(covered), len(attempted))
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

	// Scenario-level signing config, read by executeStep (a step may override).
	ctx.Data["_signingKey"] = scenario.SigningKey
	ctx.Data["_signingAlgorithm"] = scenario.SigningAlgorithm

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
		result.CRIds = mergeCRIds(scenario.CRIds, step.CRIds)
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
		Method:     step.Method,
		Expected:   step.ExpectedStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		StatusCode: 0,
	}

	// Prepare endpoint with context interpolation
	endpoint := interpolateContext(step.Endpoint, ctx)
	result.Endpoint = endpoint

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

	// Create HTTP request. An endpoint may be an absolute URL (used by MI-018 to
	// hit the untrusted-CA listener) or a path relative to WFMServer.
	reqURL := WFMServer + endpoint
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		reqURL = endpoint
	}
	req, err := http.NewRequest(step.Method, reqURL, bodyReader)
	if err != nil {
		result.Status = "fail"
		result.Reason = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")

	// RFC 9421: sign all requests (adds Signature-Input, Signature, Content-Digest)
	// unless skip_signing is true. Signing key/algorithm come from the step, then
	// the scenario, then the default device key.
	if !step.SkipSigning {
		keyPath := firstNonEmpty(step.SigningKey, ctxString(ctx, "_signingKey"), getDeviceKeyPath())
		alg := firstNonEmpty(step.SigningAlgorithm, ctxString(ctx, "_signingAlgorithm"))
		if err := signRequest(req, bodyBytes, keyPath, alg); err != nil {
			result.Status = "fail"
			result.Reason = fmt.Sprintf("Failed to sign request: %v", err)
			return result
		}
	}

	// Add custom headers from test definition (after signing so they can override if needed)
	for key, value := range step.Headers {
		req.Header.Set(key, interpolateHeaderValue(value, ctx))
	}

	// Execute request. Default client skips TLS verification (self-signed mock
	// cert); verify_tls swaps in a client that checks the server cert against
	// the fetched root CA (MI-018).
	client := tlsSkipClient()
	if step.VerifyTLS {
		vc, vcErr := caVerifyingClient()
		if vcErr != nil {
			result.Status = "fail"
			result.Reason = fmt.Sprintf("could not build CA-verifying client: %v", vcErr)
			return result
		}
		client = vc
	}
	resp, err := client.Do(req)
	if err != nil {
		if step.ExpectTransportError {
			result.Status = "pass"
			result.Reason = fmt.Sprintf("transport error as expected: %v", err)
			return result
		}
		result.Status = "fail"
		result.Reason = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()
	if step.ExpectTransportError {
		result.Status = "fail"
		result.Reason = fmt.Sprintf("expected a transport error but request succeeded (HTTP %d)", resp.StatusCode)
		return result
	}

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

	// Manifest-verdict check: some requirements are about the CLIENT's decision
	// on a desired-state manifest. The suite owns that rule (it must not assume
	// the device implements it) and encodes it in evaluateManifest so a scenario
	// can assert it without a full device-agent.
	if step.ExpectManifestRejected || step.ExpectManifestAccepted {
		reason, violated := evaluateManifest(respData, ctx)
		switch {
		case step.ExpectManifestRejected && violated:
			fmt.Printf("    ✅ manifest correctly flagged as non-conformant: %s\n", reason)
		case step.ExpectManifestRejected:
			result.Status = "fail"
			result.Reason = "expect_manifest_rejected: manifest is spec-conformant, nothing for a client to reject"
			return result
		case step.ExpectManifestAccepted && violated:
			result.Status = "fail"
			result.Reason = "expect_manifest_accepted: a conformant client would reject this manifest: " + reason
			return result
		default:
			fmt.Printf("    ✅ manifest is conformant on every integrity-relevant check; a client MUST proceed\n")
		}
	}
	// Track the highest manifestVersion seen on an accepted manifest GET, so a
	// later non-increasing version can be detected (MI-010). Only GET responses
	// are real manifests — the test-control PUT echoes a version too.
	if step.Method == "GET" && !step.ExpectManifestRejected {
		if cur, ok := extractJSONPath(respData, "manifestVersion").(float64); ok {
			if prev, _ := ctx.Data["_seenManifestVersion"].(float64); cur > prev {
				ctx.Data["_seenManifestVersion"] = cur
			}
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
		// Send the certificate as base64-encoded PEM — the "device-agent format"
		// a real WFM (Symphony) expects and stores; it rejects a raw PEM string
		// ("illegal base64 data at input byte 0") on the next signed request.
		// The mock accepts either (it tries base64 first, then raw PEM).
		return base64.StdEncoding.EncodeToString(certData), nil
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

// RFC 9421 client-side signing + TLS trust live in signing.go.

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

// Desired-state manifest rules (evaluateManifest, signedGET) live in manifest_rules.go.

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

// mergeCRIds returns the sorted, de-duplicated union of the given CR-ID lists.
func mergeCRIds(lists ...[]string) []string {
	set := map[string]bool{}
	for _, l := range lists {
		for _, id := range l {
			if s := strings.TrimSpace(id); s != "" {
				set[s] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// crIDCoverage returns (verified, referenced): CR-IDs cited by at least one
// passing step, and CR-IDs cited by any step (pass or fail).
func crIDCoverage(results []TestResult) (verified, referenced []string) {
	v := map[string]bool{}
	ref := map[string]bool{}
	for _, r := range results {
		for _, id := range r.CRIds {
			ref[id] = true
			if r.Status == "pass" {
				v[id] = true
			}
		}
	}
	return sortedKeys(v), sortedKeys(ref)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// htmlEscape mirrors the wfm-supplier report's escaping.
func htmlEscape(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(v)
}

// scenarioTally is a per-scenario pass/fail rollup for the report's summary table.
type scenarioTally struct {
	name                 string
	total, passed, faild int
}

func generateHTMLReport(results []TestResult) string {
	passCount, failCount := 0, 0
	for _, r := range results {
		if r.Status == "pass" {
			passCount++
		} else {
			failCount++
		}
	}

	// Per-scenario rollup, preserving first-seen order.
	var order []string
	tallies := map[string]*scenarioTally{}
	for _, r := range results {
		t := tallies[r.ScenarioID]
		if t == nil {
			t = &scenarioTally{name: r.ScenarioName}
			tallies[r.ScenarioID] = t
			order = append(order, r.ScenarioID)
		}
		t.total++
		if r.Status == "pass" {
			t.passed++
		} else {
			t.faild++
		}
	}

	verified, referenced := crIDCoverage(results)
	verifiedSet := map[string]bool{}
	for _, id := range verified {
		verifiedSet[id] = true
	}

	var b strings.Builder
	b.WriteString(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Device Conformance Report</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 24px; color: #1f2933; }
    h1 { font-size: 22px; margin-bottom: 4px; }
    h2 { font-size: 16px; margin: 24px 0 8px; }
    .meta { font-size: 13px; color: #555; margin-bottom: 18px; }
    .summary { margin-bottom: 10px; font-size: 15px; font-weight: bold; }
    .summary.all-pass { color: #166534; }
    .summary.has-fail { color: #b91c1c; }
    table { border-collapse: collapse; width: 100%; font-size: 13px; margin-bottom: 24px; }
    th, td { border: 1px solid #d7dde5; padding: 7px 9px; text-align: left; vertical-align: top; }
    th { background: #eef2f7; }
    tr.pass td:first-child { color: #166534; font-weight: 700; }
    tr.fail td:first-child, tr.fail td:last-child { color: #b91c1c; font-weight: 700; }
    .version-warning { margin-bottom: 14px; padding: 10px 14px; border-radius: 4px; background: #dcfce7; color: #166534; border: 1px solid #86efac; font-size: 13px; font-weight: bold; }
  </style>
</head>
<body>
  <h1>Margo Device Conformance Report</h1>
`)
	fmt.Fprintf(&b, `  <div class="meta">
    Claimed App Version: <strong>%s</strong> &nbsp;|&nbsp;
    CTT Margo Version: <strong>%s</strong> &nbsp;|&nbsp;
    Target WFM: <strong>%s</strong> &nbsp;|&nbsp;
    Run: <strong>%s</strong>
  </div>
`, htmlEscape(ClaimedAppVersion), htmlEscape(CTTMargoVersion), htmlEscape(WFMServer), time.Now().Format(time.RFC3339))

	if ClaimedAppVersion != "unknown" && ClaimedAppVersion != CTTMargoVersion {
		fmt.Fprintf(&b, `  <div class="version-warning">⚠ Version Mismatch: Claimed App Version (%s) differs from CTT Margo Version (%s)</div>
`, htmlEscape(ClaimedAppVersion), htmlEscape(CTTMargoVersion))
	}

	cls := "all-pass"
	icon := "✅"
	if failCount != 0 {
		cls, icon = "has-fail", "❌"
	}
	fmt.Fprintf(&b, `  <div class="summary %s">%s %d passed, %d failed, %d total</div>
`, cls, icon, passCount, failCount, len(results))

	// Requirements coverage
	fmt.Fprintf(&b, `  <h2>Requirements Coverage</h2>
  <div class="summary">%d requirement(s) verified by passing tests &nbsp;|&nbsp; %d referenced by tests in total</div>
  <table>
    <thead><tr><th>CR-ID</th><th>Verified</th></tr></thead>
    <tbody>
`, len(verified), len(referenced))
	for _, id := range referenced {
		rowCls, mark := "pass", "✅"
		if !verifiedSet[id] {
			rowCls, mark = "fail", "❌"
		}
		fmt.Fprintf(&b, "    <tr class=\"%s\"><td>%s</td><td>%s</td></tr>\n", rowCls, htmlEscape(id), mark)
	}
	b.WriteString("    </tbody>\n  </table>\n")

	// Scenario summary
	b.WriteString(`  <h2>Scenario Summary</h2>
  <table>
    <thead><tr><th>Scenario</th><th>Total</th><th>Passed</th><th>Failed</th></tr></thead>
    <tbody>
`)
	for _, id := range order {
		t := tallies[id]
		rowCls := "pass"
		if t.faild != 0 {
			rowCls = "fail"
		}
		fmt.Fprintf(&b, "    <tr class=\"%s\"><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>\n",
			rowCls, htmlEscape(t.name), t.total, t.passed, t.faild)
	}
	b.WriteString("    </tbody>\n  </table>\n")

	// Step details
	b.WriteString(`  <h2>Step Details</h2>
  <table>
    <thead><tr>
      <th>Status</th><th>Scenario</th><th>Step</th><th>CR-IDs</th><th>Name</th>
      <th>Method</th><th>Endpoint</th><th>Expected</th><th>Actual</th><th>Failure Reason</th>
    </tr></thead>
    <tbody>
`)
	for _, r := range results {
		rowCls, statusText := "pass", "PASS"
		if r.Status != "pass" {
			rowCls, statusText = "fail", "FAIL"
		}
		expected := ""
		if r.Expected != 0 {
			expected = fmt.Sprintf("%d", r.Expected)
		}
		fmt.Fprintf(&b, `    <tr class="%s">
      <td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td>
      <td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td>
    </tr>
`, rowCls, statusText, htmlEscape(r.ScenarioName), htmlEscape(r.StepID),
			htmlEscape(strings.Join(r.CRIds, ", ")), htmlEscape(r.StepName),
			htmlEscape(r.Method), htmlEscape(r.Endpoint), expected, r.StatusCode, htmlEscape(r.Reason))
	}
	b.WriteString("    </tbody>\n  </table>\n</body>\n</html>\n")

	return b.String()
}
