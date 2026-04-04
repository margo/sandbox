package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-faster/jx"
	"github.com/margo/sandbox/standard/cmd/mock-wfm/conformance_ogen/api"
)

// MockWFMServer implements the ogen-generated Handler interface
type MockWFMServer struct {
	deviceCapabilities sync.Map // clientID -> *api.DeviceCapabilitiesManifest
	deployments        sync.Map // clientID -> []*api.DeploymentStatusManifest
	deploymentStatuses sync.Map // clientID:deploymentID -> *api.DeploymentStatusManifest
	onboardedClients   sync.Map // certificate -> clientID
	catalogMu          sync.Mutex
	catalogSignature   string
	catalogVersion     uint64
}

// NewMockWFMServer creates a new mock server
func NewMockWFMServer() *MockWFMServer {
	return &MockWFMServer{}
}

// APIV1ClientsClientIdBundlesDigestGet retrieves bundle information
func (m *MockWFMServer) APIV1ClientsClientIdBundlesDigestGet(
	ctx context.Context,
	params api.APIV1ClientsClientIdBundlesDigestGetParams,
) (api.APIV1ClientsClientIdBundlesDigestGetRes, error) {
	clientID := params.ClientId
	digest := params.Digest

	catalog, _, err := m.catalogSnapshot()
	if err != nil {
		return nil, err
	}
	if catalog.Bundle == nil {
		return &api.APIV1ClientsClientIdBundlesDigestGetNotFound{}, nil
	}
	if catalog.Bundle.Digest != digest {
		return &api.APIV1ClientsClientIdBundlesDigestGetNotFound{}, nil
	}

	etag := quoteETag(catalog.Bundle.Digest)
	if ifNoneMatch, ok := params.IfNoneMatch.Get(); ok && matchesETag(ifNoneMatch, etag) {
		return &api.APIV1ClientsClientIdBundlesDigestGetNotModified{}, nil
	}

	fmt.Printf("✅ Retrieved bundle for client: %s, digest: %s\n", clientID, digest)

	resp := &api.APIV1ClientsClientIdBundlesDigestGetOKHeaders{
		Response: api.APIV1ClientsClientIdBundlesDigestGetOK{
			Data: bytes.NewReader(catalog.Bundle.Bytes),
		},
	}
	resp.SetETag(api.NewOptString(etag))
	resp.SetCacheControl(api.NewOptString(immutableCacheTTL))
	return resp, nil
}

// APIV1ClientsClientIdCapabilitiesPost handles device capability reporting (POST)
func (m *MockWFMServer) APIV1ClientsClientIdCapabilitiesPost(
	ctx context.Context,
	req *api.DeviceCapabilitiesManifest,
	params api.APIV1ClientsClientIdCapabilitiesPostParams,
) (api.APIV1ClientsClientIdCapabilitiesPostRes, error) {
	clientID := params.ClientId

	if req == nil {
		return nil, fmt.Errorf("request body is required")
	}

	// Store capabilities
	m.deviceCapabilities.Store(clientID, req)

	fmt.Printf("✅ Stored device capabilities (POST) for client: %s\n", clientID)
	if req.Properties.ID != "" {
		fmt.Printf("   Device ID: %s\n", req.Properties.ID)
	}

	return &api.APIV1ClientsClientIdCapabilitiesPostCreated{}, nil
}

// APIV1ClientsClientIdCapabilitiesPut handles device capability updates (PUT)
func (m *MockWFMServer) APIV1ClientsClientIdCapabilitiesPut(
	ctx context.Context,
	req *api.DeviceCapabilitiesManifest,
	params api.APIV1ClientsClientIdCapabilitiesPutParams,
) (api.APIV1ClientsClientIdCapabilitiesPutRes, error) {
	clientID := params.ClientId

	if req == nil {
		return nil, fmt.Errorf("request body is required")
	}

	// Store/update capabilities
	m.deviceCapabilities.Store(clientID, req)

	fmt.Printf("✅ Updated device capabilities (PUT) for client: %s\n", clientID)

	return &api.APIV1ClientsClientIdCapabilitiesPutCreated{}, nil
}

// APIV1ClientsClientIdDeploymentsDeploymentIdDigestGet fetches a single deployment
func (m *MockWFMServer) APIV1ClientsClientIdDeploymentsDeploymentIdDigestGet(
	ctx context.Context,
	params api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetParams,
) (api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetRes, error) {
	clientID := params.ClientId
	deploymentID := params.DeploymentId
	digest := params.Digest

	catalog, _, err := m.catalogSnapshot()
	if err != nil {
		return nil, err
	}

	deployment := catalog.findDeployment(deploymentID, digest)
	if deployment == nil {
		return &api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetNotFound{}, nil
	}

	fmt.Printf("✅ Retrieved deployment %s for client: %s, digest: %s\n", deploymentID, clientID, digest)

	resp := &api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetOKHeaders{
		Response: api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetOK{
			Data: bytes.NewReader(deployment.YAML),
		},
	}
	resp.SetETag(api.NewOptString(quoteETag(deployment.Digest)))
	resp.SetCacheControl(api.NewOptString(immutableCacheTTL))
	resp.SetVary(api.NewOptString("Accept-Encoding"))
	return resp, nil
}

// APIV1ClientsClientIdDeploymentsDeploymentIdStatusPost reports deployment status
func (m *MockWFMServer) APIV1ClientsClientIdDeploymentsDeploymentIdStatusPost(
	ctx context.Context,
	req *api.DeploymentStatusManifest,
	params api.APIV1ClientsClientIdDeploymentsDeploymentIdStatusPostParams,
) (api.APIV1ClientsClientIdDeploymentsDeploymentIdStatusPostRes, error) {
	clientID := params.ClientId
	deploymentID := params.DeploymentId

	if req == nil {
		return nil, fmt.Errorf("request body is required")
	}

	// Store deployment status
	key := fmt.Sprintf("%s:%s", clientID, deploymentID)
	m.deploymentStatuses.Store(key, req)

	fmt.Printf("✅ Updated status for deployment %s (client: %s)\n", deploymentID, clientID)

	return &api.APIV1ClientsClientIdDeploymentsDeploymentIdStatusPostOK{}, nil
}

// APIV1ClientsClientIdDeploymentsGet retrieves all deployments for a client
func (m *MockWFMServer) APIV1ClientsClientIdDeploymentsGet(
	ctx context.Context,
	params api.APIV1ClientsClientIdDeploymentsGetParams,
) (api.APIV1ClientsClientIdDeploymentsGetRes, error) {
	clientID := params.ClientId

	if accept, ok := params.Accept.Get(); ok && !acceptsManifest(accept) {
		return &api.APIV1ClientsClientIdDeploymentsGetNotAcceptable{}, nil
	}

	catalog, version, err := m.catalogSnapshot()
	if err != nil {
		return nil, err
	}
	manifest := buildManifestForClient(clientID, version, catalog)

	fmt.Printf("✅ Retrieved deployments for client: %s (deployments: %d)\n", clientID, len(manifest.Deployments))

	// Wrap in headers response
	resp := &api.UnsignedAppStateManifestHeaders{
		Response: manifest,
	}
	resp.SetETag(api.NewOptString(manifestETag(manifest)))

	if ifNoneMatch, ok := params.IfNoneMatch.Get(); ok && matchesETag(ifNoneMatch, resp.ETag.Or("")) {
		return &api.APIV1ClientsClientIdDeploymentsGetNotModified{}, nil
	}

	return resp, nil
}

// APIV1OnboardingCertificateGet returns the onboarding certificate
func (m *MockWFMServer) APIV1OnboardingCertificateGet(
	ctx context.Context,
) (*api.APIV1OnboardingCertificateGetOK, error) {
	fmt.Println("✅ Retrieved onboarding certificate")

	resp := &api.APIV1OnboardingCertificateGetOK{}
	if cert, err := readOnboardingCACertificate(); err == nil && len(cert) > 0 {
		resp.SetCertificate(api.NewOptString(base64.StdEncoding.EncodeToString(cert)))
	}

	return resp, nil
}

// APIV1OnboardingPost completes the onboarding process
func (m *MockWFMServer) APIV1OnboardingPost(
	ctx context.Context,
	req *api.APIV1OnboardingPostReq,
) (api.APIV1OnboardingPostRes, error) {
	if req == nil {
		fmt.Println("❌ Onboarding request is nil")
		resp := &api.APIV1OnboardingPostBadRequest{}
		resp.SetError(api.NewOptString("request body is required"))
		return resp, nil
	}

	if req.GetCertificate() == "" {
		resp := &api.APIV1OnboardingPostBadRequest{}
		resp.SetError(api.NewOptString("certificate is required"))
		return resp, nil
	}

	clientID := certificateClientID(req.GetCertificate())
	if existing, ok := m.onboardedClients.Load(req.GetCertificate()); ok {
		clientID = existing.(string)
	} else {
		m.onboardedClients.Store(req.GetCertificate(), clientID)
	}

	fmt.Printf("✅ Device onboarding completed. clientId=%s kind=%v\n", clientID, req.GetKind())

	resp := &api.APIV1OnboardingPostCreated{}
	resp.SetClientId(api.NewOptString(clientID))
	return resp, nil
}

// Helper to log all device capabilities (debugging)
func (m *MockWFMServer) ListDevices() {
	count := 0
	m.deviceCapabilities.Range(func(key, value interface{}) bool {
		count++
		clientID := key.(string)
		fmt.Printf("  Device %d: %s\n", count, clientID)
		return true
	})
	if count == 0 {
		fmt.Println("  (no devices registered)")
	}
}

// Helper to log all deployment statuses (debugging)
func (m *MockWFMServer) ListDeploymentStatuses() {
	count := 0
	m.deploymentStatuses.Range(func(key, value interface{}) bool {
		count++
		keyStr := key.(string)
		fmt.Printf("  Deployment %d: %s\n", count, keyStr)
		return true
	})
	if count == 0 {
		fmt.Println("  (no deployments reported)")
	}
}

func manifestETag(manifest api.UnsignedAppStateManifest) string {
	e := new(jx.Encoder)
	manifest.Encode(e)
	sum := sha256.Sum256(e.Bytes())
	return fmt.Sprintf("\"sha256:%x\"", sum)
}

func certificateClientID(certificate string) string {
	sum := sha256.Sum256([]byte(certificate))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%04x%08x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint16(b[10])<<8|uint16(b[11]),
		uint32(b[12])<<24|uint32(b[13])<<16|uint32(b[14])<<8|uint32(b[15]),
	)
}

func readOnboardingCACertificate() ([]byte, error) {
	for _, candidate := range caCertificateCandidates() {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("ca-cert.pem not found")
}

func caCertificateCandidates() []string {
	var candidates []string

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "ca-cert.pem"),
			filepath.Join(exeDir, "certs", "ca-cert.pem"),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "ca-cert.pem"),
			filepath.Join(wd, "certs", "ca-cert.pem"),
		)
	}

	return candidates
}

func acceptsManifest(header string) bool {
	for _, part := range strings.Split(header, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch mediaType {
		case "*/*", "application/*", manifestMediaType:
			return true
		}
	}

	return false
}

func (m *MockWFMServer) catalogSnapshot() (mockCatalog, uint64, error) {
	catalog, err := loadMockCatalog()
	if err != nil {
		return mockCatalog{}, 0, err
	}

	return catalog, m.nextManifestVersion(catalog.Signature), nil
}

func (m *MockWFMServer) nextManifestVersion(signature string) uint64 {
	m.catalogMu.Lock()
	defer m.catalogMu.Unlock()

	if m.catalogVersion == 0 {
		m.catalogVersion = 1
		m.catalogSignature = signature
		return m.catalogVersion
	}

	if signature != m.catalogSignature {
		m.catalogVersion++
		m.catalogSignature = signature
	}

	return m.catalogVersion
}
