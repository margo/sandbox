package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ===== MARGO DESIRED-STATE MANIFEST RULES =====
//
// The subset of desired-state rules a conformant device client MUST enforce
// before acting on a manifest. The runner owns these (it must not assume the
// device implements them); a scenario asserts the expected verdict with
// expect_manifest_rejected / expect_manifest_accepted.
//
// Add a rule here + a matching negative fixture on the mock
// (cmd/device-supplier/negative_fixtures.go) to cover another requirement.

// signedGET issues an RFC 9421-signed GET and returns status + body, fetching a
// manifest artifact exactly the way a device client would.
func signedGET(endpoint string) (int, []byte, error) {
	req, err := http.NewRequest("GET", WFMServer+endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	if err := signRequest(req, nil, getDeviceKeyPath(), ""); err != nil {
		return 0, nil, err
	}
	resp, err := tlsSkipClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}

// eachManifestRef calls fn for the bundle object and every deployment ref — the
// manifest nodes carrying a digest/sizeBytes/url triple.
func eachManifestRef(m map[string]interface{}, fn func(label string, ref map[string]interface{})) {
	if b, ok := m["bundle"].(map[string]interface{}); ok {
		fn("bundle", b)
	}
	if list, ok := m["deployments"].([]interface{}); ok {
		for i, item := range list {
			if ref, ok := item.(map[string]interface{}); ok {
				fn(fmt.Sprintf("deployments[%d]", i), ref)
			}
		}
	}
}

// evaluateManifest returns a reason and true when the manifest violates a rule a
// conformant client MUST enforce (so the client MUST abort). Each rule cites its
// CR-ID.
func evaluateManifest(data interface{}, ctx *TestContext) (string, bool) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return "response body is not a JSON object", true
	}

	// MI-010: manifestVersion MUST be strictly greater than any already seen.
	if cur, ok := m["manifestVersion"].(float64); ok {
		if prev, seen := ctx.Data["_seenManifestVersion"].(float64); seen && cur <= prev {
			return fmt.Sprintf("manifestVersion %v is not strictly greater than previously seen %v (MI-010)", cur, prev), true
		}
	}

	var reason string
	var bad bool
	eachManifestRef(m, func(label string, ref map[string]interface{}) {
		if bad {
			return
		}
		digest, _ := ref["digest"].(string)
		// MI-008 / MI-027: sha256 is the only permitted digest algorithm.
		if !strings.HasPrefix(digest, "sha256:") {
			reason, bad = fmt.Sprintf("%s.digest uses an unsupported algorithm: %q (MI-008)", label, digest), true
			return
		}
		// MI-006 / MI-009 / MI-019: the referenced artifact's sha256 MUST match
		// the digest the manifest claims.
		refURL, _ := ref["url"].(string)
		if refURL == "" {
			return
		}
		code, body, err := signedGET(refURL)
		if err != nil || code != 200 {
			reason, bad = fmt.Sprintf("%s: artifact fetch failed (HTTP %d, err %v) (MI-019)", label, code, err), true
			return
		}
		if got := "sha256:" + fmt.Sprintf("%x", sha256.Sum256(body)); got != digest {
			reason, bad = fmt.Sprintf("%s: digest mismatch — manifest says %s, content is %s (MI-006/019)", label, digest, got), true
		}
	})
	return reason, bad
}
