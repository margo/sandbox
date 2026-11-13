package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/margo/sandbox/poc/device/agent/types"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

const baseConfigYAML = `
logging: {level: DEBUG}
database: {dataDir: /tmp/agent}
miaf:
  x509: {certPath: c.pem, keyPath: k.pem}
  mis: {endpoint: "https://mis", caPath: ca.crt}
  authzPath: authz.json
wfm: {sbiUrl: "https://wfm"}
stateSeeking: {interval: 15}
runtimes:
  - type: KUBERNETES
    kubernetes: {kubeconfigPath: /tmp/kc}
capabilities: {readFromFile: caps.json}
`

// newTestSyncer loads a config (base + bundleYAML) via types.LoadConfig so
// that default handling is exercised exactly as in production.
func newTestSyncer(t *testing.T, bundleYAML string) *StateSyncer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(baseConfigYAML+bundleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := types.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return NewStateSyncer(nil, nil, cfg.StateSeeking.Interval, cfg.Bundle, zap.NewNop().Sugar())
}

func TestShouldDownloadBundle(t *testing.T) {
	digest := "sha256:abc"
	const mb = 1024 * 1024

	// NOTE: replace float32 with the real type of Bundle.SizeBytes
	size := func(n float32) *float32 { return &n }
	withDigest := &sbi.DeploymentBundleRef{Digest: &digest}
	withSize := func(n float32) *sbi.DeploymentBundleRef {
		return &sbi.DeploymentBundleRef{Digest: &digest, SizeBytes: size(n)}
	}

	tests := []struct {
		name        string
		bundleYAML  string
		deployments int
		bundle      *sbi.DeploymentBundleRef
		want        bool
	}{
		// defaults (section missing)
		{"no bundle advertised", "", 10, nil, false},
		{"default: 6 deployments uses bundle", "", 6, withDigest, true},
		{"default: exactly 5 does not use bundle", "", 5, withDigest, false},
		{"default: small bundle, few deployments", "", 2, withSize(1 * mb), false},
		{"default: bundle above 50MB", "", 2, withSize(60 * mb), true},

		// custom values
		{"custom deploymentThreshold 1",
			"bundle:\n  deploymentThreshold: 1\n", 2, withDigest, true},
		{"custom sizeThresholdMB 1",
			"bundle:\n  sizeThresholdMB: 1\n", 1, withSize(2 * mb), true},

		// zero values fall back to defaults
		{"zero deploymentThreshold -> default 5",
			"bundle:\n  deploymentThreshold: 0\n", 5, withDigest, false},
		{"zero sizeThresholdMB -> default 50",
			"bundle:\n  sizeThresholdMB: 0\n", 2, withSize(1 * mb), false},

		// negative values fall back to defaults
		{"negative deploymentThreshold: 5 deployments stays individual",
			"bundle:\n  deploymentThreshold: -3\n", 5, withDigest, false},
		{"negative deploymentThreshold: 6 deployments uses bundle",
			"bundle:\n  deploymentThreshold: -3\n", 6, withDigest, true},
		{"negative sizeThresholdMB: 1MB bundle stays individual",
			"bundle:\n  sizeThresholdMB: -10\n", 2, withSize(1 * mb), false},
		{"negative sizeThresholdMB: 60MB bundle uses bundle",
			"bundle:\n  sizeThresholdMB: -10\n", 2, withSize(60 * mb), true},
		{"both negative -> both defaults",
			"bundle:\n  deploymentThreshold: -1\n  sizeThresholdMB: -1\n",
			5, withSize(1 * mb), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ss := newTestSyncer(t, tc.bundleYAML)
			m := &sbi.UnsignedAppStateManifest{
				Deployments: make([]sbi.DeploymentManifestRef, tc.deployments),
				Bundle:      tc.bundle,
			}
			if got := ss.shouldDownloadBundle(m); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
