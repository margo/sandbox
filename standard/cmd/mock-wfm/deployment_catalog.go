package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/margo/sandbox/standard/cmd/mock-wfm/conformance_ogen/api"
	yaml "gopkg.in/yaml.v2"
)

const (
	manifestMediaType = "application/vnd.margo.manifest.v1+json"
	bundleMediaType   = "application/vnd.margo.bundle.v1+tar+gzip"
	immutableCacheTTL = "public, max-age=31536000, immutable"
)

type mockCatalog struct {
	Deployments []mockDeploymentAsset
	Bundle      *mockBundleAsset
	Signature   string
}

type mockDeploymentAsset struct {
	ID       string
	Name     string
	Filename string
	YAML     []byte
	Digest   string
}

type mockBundleAsset struct {
	Bytes  []byte
	Digest string
}

type deploymentMetadataEnvelope struct {
	Metadata struct {
		Name        string            `yaml:"name"`
		ID          string            `yaml:"id"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
}

func loadMockCatalog() (mockCatalog, error) {
	deployments, err := loadCatalogDeployments()
	if err != nil {
		return mockCatalog{}, err
	}
	if len(deployments) == 0 {
		deployments = defaultMockDeployments()
	}

	bundle, err := buildBundleAsset(deployments)
	if err != nil {
		return mockCatalog{}, err
	}

	return mockCatalog{
		Deployments: deployments,
		Bundle:      bundle,
		Signature:   catalogSignature(deployments, bundle),
	}, nil
}

func loadCatalogDeployments() ([]mockDeploymentAsset, error) {
	for _, candidate := range deploymentCatalogCandidates() {
		deployments, err := loadDeploymentsFromDir(candidate)
		if err != nil {
			return nil, err
		}
		if len(deployments) > 0 {
			return deployments, nil
		}
	}

	return nil, nil
}

func loadDeploymentsFromDir(dir string) ([]mockDeploymentAsset, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read deployments dir %q: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var deployments []mockDeploymentAsset
	seenIDs := map[string]int{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read deployment file %q: %w", name, err)
		}

		deployment := newMockDeploymentAsset(name, content)
		deployment.ID = dedupeDeploymentID(deployment.ID, deployment.Digest, seenIDs)
		deployment.Filename = deployment.ID + ".yaml"
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func deploymentCatalogCandidates() []string {
	var candidates []string

	if envDir := strings.TrimSpace(os.Getenv("MOCK_WFM_DEPLOYMENTS_DIR")); envDir != "" {
		candidates = append(candidates, envDir)
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "mock-data", "deployments"),
			filepath.Join(exeDir, "deployments"),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "mock-data", "deployments"),
			filepath.Join(wd, "deployments"),
		)
	}

	return dedupeStrings(candidates)
}

func defaultMockDeployments() []mockDeploymentAsset {
	return []mockDeploymentAsset{
		newMockDeploymentAsset("compose-sample.yaml", []byte(defaultComposeDeploymentYAML)),
	}
}

func newMockDeploymentAsset(fileName string, content []byte) mockDeploymentAsset {
	id, name := extractDeploymentIdentity(fileName, content)
	if id == "" {
		id = trimYAMLExtension(fileName)
	}
	if name == "" {
		name = id
	}

	return mockDeploymentAsset{
		ID:       id,
		Name:     name,
		Filename: id + ".yaml",
		YAML:     append([]byte(nil), content...),
		Digest:   contentDigest(content),
	}
}

func extractDeploymentIdentity(fileName string, content []byte) (id string, name string) {
	var envelope deploymentMetadataEnvelope
	if err := yaml.Unmarshal(content, &envelope); err != nil {
		fileStem := trimYAMLExtension(fileName)
		return fileStem, fileStem
	}

	fileStem := trimYAMLExtension(fileName)
	id = firstNonEmpty(
		envelope.Metadata.Annotations["id"],
		envelope.Metadata.ID,
		fileStem,
	)
	name = firstNonEmpty(envelope.Metadata.Name, id)
	return id, name
}

func dedupeDeploymentID(id, digest string, seen map[string]int) string {
	if id == "" {
		id = "deployment"
	}

	count := seen[id]
	seen[id] = count + 1
	if count == 0 {
		return id
	}

	shortDigest := strings.TrimPrefix(digest, "sha256:")
	if len(shortDigest) > 8 {
		shortDigest = shortDigest[:8]
	}
	return fmt.Sprintf("%s-%s", id, shortDigest)
}

func buildBundleAsset(deployments []mockDeploymentAsset) (*mockBundleAsset, error) {
	if len(deployments) == 0 {
		return nil, nil
	}

	sorted := append([]mockDeploymentAsset(nil), deployments...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Filename < sorted[j].Filename
	})

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Header.Name = ""
	gzipWriter.Header.Comment = ""
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()

	tarWriter := tar.NewWriter(gzipWriter)
	for _, deployment := range sorted {
		header := &tar.Header{
			Name:    deployment.Filename,
			Size:    int64(len(deployment.YAML)),
			Mode:    0644,
			ModTime: time.Unix(0, 0).UTC(),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write tar header for %q: %w", deployment.Filename, err)
		}
		if _, err := tarWriter.Write(deployment.YAML); err != nil {
			return nil, fmt.Errorf("write tar entry for %q: %w", deployment.Filename, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	bundleBytes := append([]byte(nil), buf.Bytes()...)
	return &mockBundleAsset{
		Bytes:  bundleBytes,
		Digest: contentDigest(bundleBytes),
	}, nil
}

func buildManifestForClient(clientID string, version uint64, catalog mockCatalog) api.UnsignedAppStateManifest {
	manifest := api.UnsignedAppStateManifest{
		ManifestVersion: api.ManifestVersion(float64(version)),
		Deployments:     make([]api.DeploymentManifestRef, 0, len(catalog.Deployments)),
	}

	if catalog.Bundle == nil {
		manifest.Bundle.SetToNull()
	} else {
		bundle := api.DeploymentBundleRef{}
		bundle.SetMediaType(api.NewOptString(bundleMediaType))
		bundle.SetDigest(api.NewOptString(catalog.Bundle.Digest))
		bundle.SetSizeBytes(api.NewOptFloat64(float64(len(catalog.Bundle.Bytes))))
		bundle.SetURL(api.NewOptString(bundleURL(clientID, catalog.Bundle.Digest)))
		manifest.Bundle.SetTo(bundle)
	}

	for _, deployment := range catalog.Deployments {
		ref := api.DeploymentManifestRef{
			DeploymentId: deployment.ID,
			Digest:       deployment.Digest,
			URL:          deploymentURL(clientID, deployment.ID, deployment.Digest),
		}
		ref.SetSizeBytes(api.NewOptFloat64(float64(len(deployment.YAML))))
		manifest.Deployments = append(manifest.Deployments, ref)
	}

	return manifest
}

func (c mockCatalog) findDeployment(id, digest string) *mockDeploymentAsset {
	for i := range c.Deployments {
		deployment := &c.Deployments[i]
		if deployment.ID == id && deployment.Digest == digest {
			return deployment
		}
	}

	return nil
}

func bundleURL(clientID, digest string) string {
	return fmt.Sprintf("/api/v1/clients/%s/bundles/%s", clientID, digest)
}

func deploymentURL(clientID, deploymentID, digest string) string {
	return fmt.Sprintf("/api/v1/clients/%s/deployments/%s/%s", clientID, deploymentID, digest)
}

func quoteETag(value string) string {
	return fmt.Sprintf("\"%s\"", value)
}

func matchesETag(candidate, expected string) bool {
	candidate = strings.TrimSpace(candidate)
	return candidate == expected || candidate == strings.Trim(expected, "\"")
}

func contentDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", sum)
}

func catalogSignature(deployments []mockDeploymentAsset, bundle *mockBundleAsset) string {
	h := sha256.New()
	for _, deployment := range deployments {
		_, _ = h.Write([]byte(deployment.ID))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(deployment.Digest))
		_, _ = h.Write([]byte{0})
	}
	if bundle != nil {
		_, _ = h.Write([]byte(bundle.Digest))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func trimYAMLExtension(name string) string {
	name = strings.TrimSuffix(name, ".yaml")
	name = strings.TrimSuffix(name, ".yml")
	return name
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

const defaultComposeDeploymentYAML = `apiVersion: margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
  name: mock-compose-sample
  namespace: default
  annotations:
    id: 11111111-1111-1111-1111-111111111111
spec:
  appPackageRef:
    id: mock-compose-sample
  deploymentProfile:
    type: compose
    components:
      - name: mock-compose-sample
        properties:
          packageLocation: https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nginx-flask-mysql/compose.yaml
          wait: true
`
