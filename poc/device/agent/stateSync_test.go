package main

import (
	"testing"

	"github.com/margo/sandbox/poc/device/agent/types"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

func TestShouldDownloadBundle(t *testing.T) {
	digest := "sha256:abc"
	const mb = 1024 * 1024

	// NOTE: replace float32 with the real type of Bundle.SizeBytes
	// (check: grep -rn "SizeBytes" standard/generatedCode/wfm/sbi/)
	size := func(n float32) *float32 { return &n }

	tests := []struct {
		name        string
		cfg         types.BundleConfig
		deployments int
		bundle      *sbi.DeploymentBundleRef
		want        bool
	}{
		{"no bundle advertised", types.BundleConfig{}, 10, nil, false},
		{"count above default threshold", types.BundleConfig{}, 6,
			&sbi.DeploymentBundleRef{Digest: &digest}, true},
		{"count equal to threshold is NOT bundle", types.BundleConfig{}, 5,
			&sbi.DeploymentBundleRef{Digest: &digest}, false},
		{"small count, small size", types.BundleConfig{}, 2,
			&sbi.DeploymentBundleRef{Digest: &digest, SizeBytes: size(1 * mb)}, false},
		{"small count, size above 50MB", types.BundleConfig{}, 2,
			&sbi.DeploymentBundleRef{Digest: &digest, SizeBytes: size(60 * mb)}, true},
		{"custom threshold of 1", types.BundleConfig{DeploymentThreshold: 1}, 2,
			&sbi.DeploymentBundleRef{Digest: &digest}, true},
		{"custom size threshold of 1MB", types.BundleConfig{SizeThresholdMB: 1}, 1,
			&sbi.DeploymentBundleRef{Digest: &digest, SizeBytes: size(2 * mb)}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ss := NewStateSyncer(nil, nil, 15, tc.cfg, zap.NewNop().Sugar())
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