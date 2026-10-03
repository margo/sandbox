package main

// MIAF (mTLS/SPIFFE) listener for the device-supplier mock WFM.
//
// This is purely additive: the legacy RFC 9421 listener (main.go, port 3001)
// and its handlers are completely untouched. This file adds a THIRD listener
// (alongside the existing MI-018 untrusted-cert one) on its own port, serving
// the NEW spec-shaped paths (no /clients/{clientId} prefix — the caller's
// identity comes from its mTLS client certificate's SPIFFE ID instead of a
// path segment issued at onboarding, since MIAF has no onboarding call at
// all). It reuses every existing business-logic helper (buildDeploymentYAML,
// buildBundleArchive, validateRequest, applyNegativeFixture, respondJSON,
// etc.) unchanged — only identity resolution and URL shape differ from the
// legacy handlers in main.go.
//
// Entirely opt-in: if MIAF_SERVER_CERT/KEY/CA aren't set (or don't exist),
// startMIAFServer logs one line and returns — every existing regression run
// (run_tests.go, the legacy listener) is unaffected.

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

const DefaultMIAFPort = ":3003"

// miafClients holds MIAF-identified clients (keyed by SPIFFE ID) separately
// from the legacy `clients` map (keyed by onboarding-issued clientId) — the
// two flows never share an identity namespace, matching the spec: MIAF has
// no clientId concept at all.
var (
	miafClients   = make(map[string]ClientData)
	miafClientsMu sync.RWMutex

	// miafKnownDeviceIDs tracks which {deviceId}s have already had capabilities
	// reported, separately from the client/identity map above. Capabilities are
	// scoped per-deviceId (PUT /capabilities/{deviceId}), not per-caller — a
	// single WFM-Client SPIFFE identity can front several deviceIds (e.g. a
	// see-thru gateway and its children), so "is this a create (201) or an
	// update (200)" must key on deviceId, not on the calling SPIFFE ID.
	miafKnownDeviceIDs   = make(map[string]bool)
	miafKnownDeviceIDsMu sync.Mutex
)

// spiffeIDFromRequest extracts the caller's SPIFFE ID from its already
// chain-validated (tls.RequireAndVerifyClientCert) client certificate. The
// spec requires exactly one URI SAN on an SVID; a cert with zero or multiple
// is rejected here rather than trusting an ambiguous identity.
func spiffeIDFromRequest(r *http.Request) (string, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "", fmt.Errorf("no client certificate presented")
	}
	uris := r.TLS.PeerCertificates[0].URIs
	if len(uris) != 1 {
		return "", fmt.Errorf("SVID must carry exactly one URI SAN, got %d", len(uris))
	}
	if uris[0].Scheme != "spiffe" {
		return "", fmt.Errorf("URI SAN is not a spiffe:// URI: %s", uris[0].String())
	}
	return uris[0].String(), nil
}

// getOrCreateMIAFClient auto-provisions a client record on its first
// authenticated request. There is no onboarding call in MIAF — a chain- and
// (optionally) allowlist-validated mTLS connection IS the authorization, so
// the mock creates state lazily instead of requiring a prior registration step.
func getOrCreateMIAFClient(spiffeID string) ClientData {
	miafClientsMu.Lock()
	defer miafClientsMu.Unlock()
	client, exists := miafClients[spiffeID]
	if !exists {
		// Matches the legacy onboarding handler's seeding (main.go ~line 962):
		// a brand-new client starts with the default deployment already
		// assigned, at manifestVersion 1 — MIAF has no onboarding call to do
		// this at, so it happens here instead, on first authenticated contact.
		client = ClientData{
			ID:              spiffeID,
			OnboardedAt:     time.Now(),
			DeploymentsData: []string{defaultDeploymentID},
			ManifestVersion: 1,
		}
		miafClients[spiffeID] = client
		mu.Lock()
		ensureDefaultDeployment(spiffeID)
		mu.Unlock()
	}
	return client
}

// buildStateManifestMIAF mirrors buildStateManifest (main.go) but with the
// new spec-shaped, clientId-free URLs (Part 5.4 of
// CONFORMANCE_FLOWS_AND_MIAF_MIGRATION.md). buildBundleArchive/
// buildDeploymentYAML/sha256Hex are reused as-is — they don't embed URLs
// themselves, so nothing about them is legacy-specific.
func buildStateManifestMIAF(deploymentIDs []string, manifestVersion int, baseURL string) (map[string]interface{}, string, error) {
	refs := make([]interface{}, 0, len(deploymentIDs))
	bundle := interface{}(nil)

	if len(deploymentIDs) > 0 {
		bundleBytes, err := buildBundleArchive("", deploymentIDs, baseURL)
		if err != nil {
			return nil, "", err
		}
		bundleDigest := sha256Hex(bundleBytes)
		bundle = map[string]interface{}{
			"mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
			"digest":    "sha256:" + bundleDigest,
			"sizeBytes": len(bundleBytes),
			"url":       fmt.Sprintf("/api/v1/bundles/sha256:%s", bundleDigest),
		}
		for _, deploymentID := range deploymentIDs {
			yamlBytes := buildDeploymentYAML("", deploymentID, baseURL)
			deploymentDigest := sha256Hex(yamlBytes)
			refs = append(refs, map[string]interface{}{
				"deploymentId": deploymentID,
				"digest":       "sha256:" + deploymentDigest,
				"sizeBytes":    len(yamlBytes),
				"url":          fmt.Sprintf("/api/v1/deployments/%s/sha256:%s", deploymentID, deploymentDigest),
			})
		}
	}

	manifest := map[string]interface{}{
		"manifestVersion": manifestVersion,
		"bundle":          bundle,
		"deployments":     refs,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	return manifest, sha256Hex(manifestBytes), nil
}

// PUT /v1alpha2/margo/api/v1/capabilities/{deviceId}
func handleMIAFPutCapabilities(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	deviceID := mux.Vars(r)["deviceId"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid JSON body"})
		return
	}

	// Reuses the same data-driven validation as the legacy endpoint — the
	// capabilities body shape didn't change under MIAF, only how the caller
	// is identified.
	errors := validateRequest("POST_capabilities", body)
	if len(errors) > 0 {
		statusCode, payload := validationErrorResponse("POST_capabilities", errors)
		respondJSON(w, statusCode, payload)
		return
	}

	client := getOrCreateMIAFClient(spiffeID)
	client.Capabilities = body
	miafClientsMu.Lock()
	miafClients[spiffeID] = client
	miafClientsMu.Unlock()

	miafKnownDeviceIDsMu.Lock()
	wasKnown := miafKnownDeviceIDs[deviceID]
	miafKnownDeviceIDs[deviceID] = true
	miafKnownDeviceIDsMu.Unlock()

	log.Printf("[MIAF/Capabilities] accepted for %s (deviceId=%s)", spiffeID, deviceID)

	// Extract trust domain from the SPIFFE ID (spiffe://<trust-domain>/...)
	trustDomain := ""
	if len(spiffeID) > len("spiffe://") {
		rest := spiffeID[len("spiffe://"):]
		if idx := strings.Index(rest, "/"); idx > 0 {
			trustDomain = rest[:idx]
		} else {
			trustDomain = rest
		}
	}

	// Include _client_cert so the test runner can assert on the validated SPIFFE
	// ID and trust domain (scenario-miaf-identity-mtls steps 1 and 2).
	resp := map[string]interface{}{
		"status": "capabilities_received",
		"_client_cert": map[string]string{
			"spiffe_id":    spiffeID,
			"trust_domain": trustDomain,
		},
	}

	// Status code distinguishes create (201) vs update (200) per spec (Part 5.4).
	if wasKnown {
		respondJSON(w, 200, resp)
	} else {
		respondJSON(w, 201, resp)
	}
}

// DELETE /v1alpha2/margo/api/v1/capabilities/{deviceId}
func handleMIAFDeleteCapabilities(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	miafClientsMu.Lock()
	_, exists := miafClients[spiffeID]
	delete(miafClients, spiffeID)
	miafClientsMu.Unlock()
	if !exists {
		respondJSON(w, 404, ResponseError{Error: "device not found"})
		return
	}
	w.WriteHeader(204)
}

// GET /v1alpha2/margo/api/v1/deployments
func handleMIAFGetDeployments(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	if !acceptsManifest(r.Header.Get("Accept")) {
		w.WriteHeader(406)
		return
	}

	client := getOrCreateMIAFClient(spiffeID)
	manifestVersion := client.ManifestVersion
	if manifestVersion == 0 {
		manifestVersion = 1
	}
	manifest, etag, err := buildStateManifestMIAF(client.DeploymentsData, manifestVersion, requestBaseURL(r))
	if err != nil {
		respondJSON(w, 500, ResponseError{Error: "Failed to build deployment manifest"})
		return
	}

	if client.NegativeFixture != FixtureNone {
		manifest = applyNegativeFixture(manifest, client.NegativeFixture)
		if b, mErr := json.Marshal(manifest); mErr == nil {
			etag = sha256Hex(b)
		}
	}

	if normalizeETag(r.Header.Get("If-None-Match")) == etag {
		w.Header().Set("ETag", quoteETag(etag))
		w.WriteHeader(304)
		return
	}

	// Attach _client_cert metadata so the test runner can assert on the
	// validated SPIFFE ID (scenario-miaf-identity-mtls step 2). This field is
	// not part of the Margo spec; a real device MUST ignore unknown fields.
	trustDomain := ""
	if len(spiffeID) > len("spiffe://") {
		rest := spiffeID[len("spiffe://"):]
		if idx := strings.Index(rest, "/"); idx > 0 {
			trustDomain = rest[:idx]
		} else {
			trustDomain = rest
		}
	}
	manifest["_client_cert"] = map[string]string{
		"spiffe_id":    spiffeID,
		"trust_domain": trustDomain,
	}

	w.Header().Set("Content-Type", "application/vnd.margo.manifest.v1+json")
	w.Header().Set("ETag", quoteETag(etag))
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(manifest)
}

// GET /v1alpha2/margo/api/v1/bundles/{digest}
func handleMIAFGetBundle(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	digest := mux.Vars(r)["digest"]
	client := getOrCreateMIAFClient(spiffeID)

	bundleBytes, err := buildBundleArchive("", client.DeploymentsData, requestBaseURL(r))
	if err != nil {
		respondJSON(w, 500, ResponseError{Error: "Failed to build deployment bundle"})
		return
	}
	expectedDigest := sha256Hex(bundleBytes)
	if strings.TrimPrefix(digest, "sha256:") != expectedDigest {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Bundle not found for digest: %s", digest)})
		return
	}
	if normalizeETag(r.Header.Get("If-None-Match")) == digest {
		w.Header().Set("ETag", quoteETag(digest))
		w.WriteHeader(304)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.margo.bundle.v1+tar+gzip")
	w.Header().Set("ETag", quoteETag(digest))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.WriteHeader(200)
	w.Write(bundleBytes)
}

// GET /v1alpha2/margo/api/v1/deployments/{deploymentId}/{digest}
func handleMIAFGetDeploymentManifest(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	vars := mux.Vars(r)
	deploymentID, digest := vars["deploymentId"], vars["digest"]
	client := getOrCreateMIAFClient(spiffeID)

	found := false
	for _, id := range client.DeploymentsData {
		if id == deploymentID {
			found = true
			break
		}
	}
	if !found {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Deployment not found: %s", deploymentID)})
		return
	}

	yamlBytes := buildDeploymentYAML("", deploymentID, requestBaseURL(r))
	expectedDigest := sha256Hex(yamlBytes)
	if strings.TrimPrefix(digest, "sha256:") != expectedDigest {
		respondJSON(w, 404, ResponseError{Error: fmt.Sprintf("Deployment not found for digest: %s", digest)})
		return
	}
	if normalizeETag(r.Header.Get("If-None-Match")) == digest {
		w.Header().Set("ETag", quoteETag(digest))
		w.WriteHeader(304)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("ETag", quoteETag(digest))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Vary", "Accept-Encoding")
	w.WriteHeader(200)
	w.Write(yamlBytes)
}

// POST /v1alpha2/margo/api/v1/deployments/{deploymentId}/status
func handleMIAFPostStatus(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}
	deploymentID := mux.Vars(r)["deploymentId"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		respondJSON(w, 400, ResponseError{Error: "Invalid JSON body"})
		return
	}

	if bodyDeploymentID, ok := getFieldValue(body, "deploymentId"); ok {
		if s, ok := bodyDeploymentID.(string); !ok || s != deploymentID {
			respondJSON(w, 422, ResponseError{Status: "validation_failed", Errors: []ValidationError{
				{RuleID: "status-path-001", Error: "deploymentId in body must match deploymentId in path"},
			}})
			return
		}
	}

	// rc.3 requirement (CONFORMANCE_FLOWS_AND_MIAF_MIGRATION.md Part 5.8):
	// adoptedManifestVersion is now required on every status report.
	if _, ok := getFieldValue(body, "adoptedManifestVersion"); !ok {
		respondJSON(w, 422, ResponseError{Status: "validation_failed", Errors: []ValidationError{
			{RuleID: "status-miaf-001", Error: "adoptedManifestVersion is required"},
		}})
		return
	}

	errors := validateRequest("POST_status", body)
	if len(errors) > 0 {
		statusCode, payload := validationErrorResponse("POST_status", errors)
		respondJSON(w, statusCode, payload)
		return
	}

	mu.Lock()
	key := deploymentKey(spiffeID, deploymentID)
	deployment, exists := deployments[key]
	if !exists {
		deployment = DeploymentData{ID: deploymentID, ClientID: spiffeID}
	}
	deployment.StatusHistory = append(deployment.StatusHistory, body)
	deployments[key] = deployment
	mu.Unlock()

	log.Printf("[MIAF/Status] update for deployment %s from %s", deploymentID, spiffeID)
	respondJSON(w, 200, map[string]string{"acknowledgement": "received"})
}

// PUT /v1alpha2/margo/api/v1/test/deployments — MIAF equivalent of
// handleTestSetDeployments (main.go), keyed by SPIFFE ID instead of clientId.
// NOT part of the Margo spec: lets the suite's own self-tests (device-mi008,
// -mi006, -mi019, -mi010, -mi025/026) arm a negative fixture or script a
// desired-state timeline against a real device-agent under mTLS, exactly the
// way the legacy test-control endpoint does for RFC 9421 clients.
func handleMIAFTestSetDeployments(w http.ResponseWriter, r *http.Request) {
	spiffeID, err := spiffeIDFromRequest(r)
	if err != nil {
		respondJSON(w, 401, ResponseError{Error: err.Error()})
		return
	}

	var body struct {
		DeploymentIDs        []string        `json:"deploymentIds"`
		NegativeFixture      NegativeFixture `json:"negativeFixture,omitempty"`
		ResetManifestVersion bool            `json:"resetManifestVersion,omitempty"`
	}
	if decErr := json.NewDecoder(r.Body).Decode(&body); decErr != nil {
		respondJSON(w, 400, ResponseError{Error: `Invalid JSON body: expected {"deploymentIds": [...], "negativeFixture"?: "<name>", "resetManifestVersion"?: true}`})
		return
	}
	if !isKnownFixture(body.NegativeFixture) {
		respondJSON(w, 400, ResponseError{Error: fmt.Sprintf("Unknown negativeFixture: %q (see negative_fixtures.go)", body.NegativeFixture)})
		return
	}
	newIDs := body.DeploymentIDs
	if newIDs == nil {
		newIDs = []string{}
	}

	miafClientsMu.Lock()
	client := miafClients[spiffeID]
	if client.ID == "" {
		client = ClientData{ID: spiffeID, OnboardedAt: time.Now()}
	}
	if client.ManifestVersion == 0 || body.ResetManifestVersion {
		// resetManifestVersion is not part of the Margo spec — it exists only
		// so a scenario that (like the pre-MIAF flow's own dedicated onboarded
		// client) needs a known, isolated starting point can establish one
		// itself, regardless of what earlier scenarios did to this suite's
		// single shared identity under MIAF (which has no per-run onboarding
		// to naturally reset state at).
		// Always reset to 1 then unconditionally increment to 2, so callers
		// get a deterministic starting version regardless of prior state
		// (e.g. if deployments were already empty the normal !stringSetsEqual
		// branch would not fire, leaving the version at 1 instead of 2).
		client.ManifestVersion = 2
		client.DeploymentsData = newIDs
		for _, id := range newIDs {
			ensureDeployment(spiffeID, id)
		}
	} else if !stringSetsEqual(client.DeploymentsData, newIDs) {
		client.ManifestVersion++
		client.DeploymentsData = newIDs
		for _, id := range newIDs {
			ensureDeployment(spiffeID, id)
		}
	}
	client.NegativeFixture = body.NegativeFixture
	miafClients[spiffeID] = client
	version := client.ManifestVersion
	miafClientsMu.Unlock()

	log.Printf("[MIAF/TestControl] %s desired state set to %v (manifestVersion=%d, negativeFixture=%q)", spiffeID, newIDs, version, body.NegativeFixture)

	respondJSON(w, 200, map[string]interface{}{
		"deployments":     newIDs,
		"manifestVersion": version,
		"negativeFixture": body.NegativeFixture,
	})
}

// startMIAFServer registers the new-shape routes on the shared router and
// starts a third listener presenting server-side mTLS. Entirely opt-in: with
// no cert/key/CA configured it logs one line and returns, so every existing
// invocation of this binary (regression runs, run_tests.go) is unaffected.
func startMIAFServer(router *mux.Router) {
	certFile := os.Getenv("MIAF_SERVER_CERT")
	keyFile := os.Getenv("MIAF_SERVER_KEY")
	caFile := os.Getenv("MIAF_TRUST_CA")
	if certFile == "" || keyFile == "" || caFile == "" {
		log.Printf("[MIAF] listener disabled (set MIAF_SERVER_CERT/MIAF_SERVER_KEY/MIAF_TRUST_CA to enable)")
		return
	}
	if _, err := os.Stat(certFile); err != nil {
		log.Printf("[MIAF] listener disabled (%s not found)", certFile)
		return
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		log.Printf("[MIAF] listener disabled (cannot read trust CA %s: %v)", caFile, err)
		return
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Printf("[MIAF] listener disabled (no valid certs in %s)", caFile)
		return
	}

	router.HandleFunc("/v1alpha2/margo/api/v1/capabilities/{deviceId}", handleMIAFPutCapabilities).Methods("PUT")
	router.HandleFunc("/v1alpha2/margo/api/v1/capabilities/{deviceId}", handleMIAFDeleteCapabilities).Methods("DELETE")
	router.HandleFunc("/v1alpha2/margo/api/v1/deployments", handleMIAFGetDeployments).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/bundles/{digest}", handleMIAFGetBundle).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/deployments/{deploymentId}/{digest}", handleMIAFGetDeploymentManifest).Methods("GET")
	router.HandleFunc("/v1alpha2/margo/api/v1/deployments/{deploymentId}/status", handleMIAFPostStatus).Methods("POST")
	router.HandleFunc("/v1alpha2/margo/api/v1/test/deployments", handleMIAFTestSetDeployments).Methods("PUT")

	port := os.Getenv("MIAF_PORT")
	if port == "" {
		port = DefaultMIAFPort
	}

	tlsConfig := &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
		MinVersion: tls.VersionTLS12, // TLS 1.3 default per spec; negotiated automatically, 1.2 allowed as fallback
	}
	server := &http.Server{Addr: port, Handler: router, TLSConfig: tlsConfig}

	go func() {
		log.Printf("🔐 MIAF (mTLS) listener on https://localhost%s", port)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("[MIAF] listener stopped: %v", err)
		}
	}()
}
