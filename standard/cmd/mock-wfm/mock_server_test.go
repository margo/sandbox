package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/margo/sandbox/standard/cmd/mock-wfm/conformance_ogen/api"
)

func TestDeploymentsManifestIncludesContentAddressableRefs(t *testing.T) {
	server := NewMockWFMServer()

	res, err := server.APIV1ClientsClientIdDeploymentsGet(context.Background(), api.APIV1ClientsClientIdDeploymentsGetParams{
		ClientId: "client-1",
	})
	if err != nil {
		t.Fatalf("APIV1ClientsClientIdDeploymentsGet returned error: %v", err)
	}

	resp, ok := res.(*api.UnsignedAppStateManifestHeaders)
	if !ok {
		t.Fatalf("unexpected response type %T", res)
	}

	if len(resp.Response.Deployments) == 0 {
		t.Fatal("expected non-empty deployments list")
	}
	if resp.Response.Bundle.IsNull() {
		t.Fatal("expected non-null bundle for non-empty deployment list")
	}

	bundle, ok := resp.Response.Bundle.Get()
	if !ok {
		t.Fatal("expected bundle value to be set")
	}
	if bundle.Digest.Or("") == "" {
		t.Fatal("expected bundle digest to be populated")
	}
	if bundle.URL.Or("") == "" {
		t.Fatal("expected bundle URL to be populated")
	}
}

func TestDeploymentAndBundleBodiesMatchAdvertisedDigests(t *testing.T) {
	server := NewMockWFMServer()

	manifestRes, err := server.APIV1ClientsClientIdDeploymentsGet(context.Background(), api.APIV1ClientsClientIdDeploymentsGetParams{
		ClientId: "client-1",
	})
	if err != nil {
		t.Fatalf("manifest request failed: %v", err)
	}

	manifestResp := manifestRes.(*api.UnsignedAppStateManifestHeaders)
	deploymentRef := manifestResp.Response.Deployments[0]
	bundleRef, _ := manifestResp.Response.Bundle.Get()

	deploymentRes, err := server.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGet(context.Background(), api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetParams{
		ClientId:     "client-1",
		DeploymentId: deploymentRef.DeploymentId,
		Digest:       deploymentRef.Digest,
	})
	if err != nil {
		t.Fatalf("deployment request failed: %v", err)
	}

	deploymentResp, ok := deploymentRes.(*api.APIV1ClientsClientIdDeploymentsDeploymentIdDigestGetOKHeaders)
	if !ok {
		t.Fatalf("unexpected deployment response type %T", deploymentRes)
	}

	deploymentBytes, err := io.ReadAll(deploymentResp.Response)
	if err != nil {
		t.Fatalf("read deployment body: %v", err)
	}
	if digest := fmt.Sprintf("sha256:%x", sha256.Sum256(deploymentBytes)); digest != deploymentRef.Digest {
		t.Fatalf("deployment digest mismatch: got %s want %s", digest, deploymentRef.Digest)
	}

	bundleRes, err := server.APIV1ClientsClientIdBundlesDigestGet(context.Background(), api.APIV1ClientsClientIdBundlesDigestGetParams{
		ClientId: "client-1",
		Digest:   bundleRef.Digest.Or(""),
	})
	if err != nil {
		t.Fatalf("bundle request failed: %v", err)
	}

	bundleResp, ok := bundleRes.(*api.APIV1ClientsClientIdBundlesDigestGetOKHeaders)
	if !ok {
		t.Fatalf("unexpected bundle response type %T", bundleRes)
	}

	bundleBytes, err := io.ReadAll(bundleResp.Response)
	if err != nil {
		t.Fatalf("read bundle body: %v", err)
	}
	if digest := fmt.Sprintf("sha256:%x", sha256.Sum256(bundleBytes)); digest != bundleRef.Digest.Or("") {
		t.Fatalf("bundle digest mismatch: got %s want %s", digest, bundleRef.Digest.Or(""))
	}
}

func TestOnboardingAcceptsSpecGenericAPIVersion(t *testing.T) {
	server := NewMockWFMServer()
	req := &api.APIV1OnboardingPostReq{}
	req.SetApiVersion("demo.test/v1")
	req.SetKind(api.APIV1OnboardingPostReqKindOnboardingRequest)
	req.SetCertificate("device-cert")

	res, err := server.APIV1OnboardingPost(context.Background(), req)
	if err != nil {
		t.Fatalf("onboarding returned error: %v", err)
	}

	if _, ok := res.(*api.APIV1OnboardingPostCreated); !ok {
		t.Fatalf("expected created response, got %T", res)
	}
}
