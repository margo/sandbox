# `trustbundle` Package

## Purpose

The `trustbundle` package retrieves [SPIFFE](https://spiffe.io/) trust bundles from a **Margo Infrastructure Service (MIS)**. It implements a three-tier resilience strategy:

1. **Discovery**: fetches the discovery document from `DiscoveryURL` and uses its `trustBundleUri` and `trustDomain` to fetch the bundle.
2. **URI Fallback**: fetches the bundle from `TrustBundleURI` if discovery fails or is not configured.
3. **Static Fallback**: returns the operator-supplied `StaticBundle` if the network sources fail.

ETag-based caching (`HTTP 304 Not Modified`) is supported to avoid redundant transfers.

---

## API Reference

### `New(cfg TrustBundleGetterConfig) (Getter, error)`

Factory function. Validates all inputs and returns a `Getter` implementation.

```go
type TrustBundleGetterConfig struct {
    DiscoveryURL   string
    MISRootCA      []byte
    MISTrustPins   []string
    TrustBundleURI string
    StaticBundle   []byte
    TrustDomain    string
}
```

| Field | Required | Description |
|---|---|---|
| `DiscoveryURL` | ❌ | Full absolute HTTPS URL of the MIS discovery document (e.g. `https://192.0.2.10/.well-known/margo`). When empty, discovery is skipped. |
| `MISRootCA` | ⚠️ | PEM-encoded CA certificate passed to the MIS client. Used for DNS-named MIS URLs. |
| `MISTrustPins` | ⚠️ | Base64-encoded SHA-256 SPKI pins (RFC 7469 §2.4). Passed to the MIS client. Used for IP-addressed MIS URLs. |
| `TrustBundleURI` | ❌ | Full absolute HTTPS URL of the SPIFFE bundle. Used when discovery fails or `DiscoveryURL` is not set. |
| `StaticBundle` | ✅ | Operator-supplied SPIFFE trust bundle in JWKS JSON format. Used as the final fallback. |
| `TrustDomain` | ✅ | Trust domain returned to the caller when the bundle does not come from discovery. |

`MISRootCA` and `MISTrustPins` are both passed to the MIS client unchanged. The client selects the trust mode per URL. Neither takes precedence over the other at this level.

**Valid configurations:**
- **Discovery mode:** `DiscoveryURL` + trust anchor + `StaticBundle` + `TrustDomain`
- **URI mode:** `TrustBundleURI` + trust anchor + `StaticBundle` + `TrustDomain`
- **Discovery and URI mode:** `DiscoveryURL` + `TrustBundleURI` + trust anchor + `StaticBundle` + `TrustDomain`
- **Static bundle mode:** `StaticBundle` + `TrustDomain` (no network configuration)

`StaticBundle` and `TrustDomain` are always required.

Returns a descriptive validation error if any input is invalid.

---

### `GetTrustBundle(ctx context.Context, ietag string) (trustDomain string, bundle []byte, etag string, err error)`

Core retrieval method on the `Getter` interface. `ietag` is the ETag from the previous successful retrieval, or `""` if none.

| Result | Returns |
|---|---|
| **HTTP 200** (network source) | `(trustDomain, bundleBytes, etag, nil)` |
| **HTTP 304 Not Modified** | `(trustDomain, nil, "", ErrNotModified)` |
| **Static fallback** | `(trustDomain, staticBundle, "", nil)` |

**Retrieval strategy:**

1. **Discovery:** If `DiscoveryURL` is set, `GET` the discovery document (no ETag). Use its `trustBundleUri` to fetch the bundle. On success, the `trustDomain` from the discovery document is returned together with the bundle.
2. **URI Fallback:** If discovery is not configured, fails, returns non-200, has no `trustBundleUri`, or the bundle fails validation, fetch from `TrustBundleURI`. The configured `TrustDomain` is returned.
3. **Static Fallback:** If the previous steps fail, return `StaticBundle` with the configured `TrustDomain`.

> **Important:** HTTP 304 short-circuits the fallback chain. The static bundle is **not** attempted on 304, since the cached network bundle is authoritative. The trust domain on 304 is the discovered one when the 304 came from the discovered bundle, and the configured one when it came from `TrustBundleURI`.

> **Note:** Because `StaticBundle` is required, the static fallback always succeeds. `GetTrustBundle` therefore does not return an error for source failures. The reasons for falling back are not returned to the caller.

---

### `ErrNotModified`

```go
var ErrNotModified = errors.New("trust bundle not modified")
```

Returned by `GetTrustBundle` when a network source responds with HTTP 304, indicating the caller's cached bundle is still current.

---

## Usage Examples

### Discovery Only

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/margo/sandbox/shared-lib/mis/trustbundle"
)

func main() {
    rootCA, err := os.ReadFile("/etc/mis/ca.pem")
    if err != nil {
        log.Fatal(err)
    }

    staticBundle, err := os.ReadFile("/etc/mis/fallback-bundle.jwks")
    if err != nil {
        log.Fatal(err)
    }

    getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
        DiscoveryURL: "https://mis.example.com/.well-known/margo",
        MISRootCA:    rootCA,
        StaticBundle: staticBundle,
        TrustDomain:  "margo.org",
    })
    if err != nil {
        log.Fatalf("invalid config: %v", err)
    }

    trustDomain, bundle, etag, err := getter.GetTrustBundle(context.Background(), "")
    if err != nil {
        log.Fatalf("failed to get trust bundle: %v", err)
    }

    fmt.Printf("Trust domain: %s\nETag: %s\nBundle: %s\n", trustDomain, etag, bundle)
}
```

---

### Trust Pins Instead of Root CA

```go
staticBundle, _ := os.ReadFile("/etc/mis/fallback-bundle.jwks")

getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
    DiscoveryURL: "https://192.0.2.10/.well-known/margo",
    MISTrustPins: []string{"sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
    StaticBundle: staticBundle,
    TrustDomain:  "margo.org",
})
if err != nil {
    log.Fatalf("invalid config: %v", err)
}
```

---

### ETag Caching: Avoid Redundant Fetches

```go
var cachedBundle []byte
var cachedETag   string
var cachedDomain string

func refreshBundle(ctx context.Context, getter trustbundle.Getter) error {
    domain, bundle, etag, err := getter.GetTrustBundle(ctx, cachedETag)
    if err != nil {
        if errors.Is(err, trustbundle.ErrNotModified) {
            // Server confirmed our cache is still valid. Nothing to do.
            return nil
        }
        return fmt.Errorf("bundle refresh failed: %w", err)
    }

    cachedBundle = bundle
    cachedETag   = etag
    cachedDomain = domain
    return nil
}
```

---

### Full Resilience: Discovery, URI, and Static

```go
rootCA, _       := os.ReadFile("/etc/mis/ca.pem")
staticBundle, _ := os.ReadFile("/etc/mis/fallback-bundle.jwks")

getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
    DiscoveryURL:   "https://mis.example.com/.well-known/margo",
    MISRootCA:      rootCA,
    TrustBundleURI: "https://mis.example.com/.well-known/spiffe/bundle.json",
    StaticBundle:   staticBundle,
    TrustDomain:    "margo.org",
})
if err != nil {
    log.Fatalf("config error: %v", err)
}

trustDomain, bundle, etag, err := getter.GetTrustBundle(context.Background(), "")
if err != nil {
    log.Fatalf("failed: %v", err)
}
```

---

### Static Bundle Only (No Network)

```go
staticBundle, _ := os.ReadFile("/etc/mis/fallback-bundle.jwks")

getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
    StaticBundle: staticBundle,
    TrustDomain:  "margo.org",
})
if err != nil {
    log.Fatalf("config error: %v", err)
}

trustDomain, bundle, _, err := getter.GetTrustBundle(context.Background(), "")
if err != nil {
    log.Fatalf("failed: %v", err)
}
```

---

## Error Handling Reference

| Scenario | Error |
|---|---|
| HTTP 304 from a network source | `trustbundle.ErrNotModified` |
| `DiscoveryURL` or `TrustBundleURI` set without a trust anchor | Validation error from `New()` |
| Trust anchor set without `DiscoveryURL` or `TrustBundleURI` | Validation error from `New()` |
| Neither a network source nor `StaticBundle` configured | Validation error from `New()` |
| `StaticBundle` missing | Validation error from `New()` |
| `StaticBundle` fails bundle validation | Wrapped validation error from `New()` |
| `TrustDomain` missing or invalid | Validation error from `New()` |
| `DiscoveryURL` or `TrustBundleURI` is not an absolute HTTPS URL | Validation error from `New()` |
| Invalid `MISRootCA` or `MISTrustPins` | Validation error from `New()` |
| Network source fails or returns an invalid bundle | Falls back silently; no error returned from `GetTrustBundle` |

---

## Bundle Validation

Every bundle fetched from the network, and the `StaticBundle` supplied at construction, is validated with `validators.ValidateSpiffeTrustBundle`. A bundle must satisfy these rules:

1. Must be valid JSON containing a `"keys"` array (JWKS format).
2. Must contain at least one X.509 trust anchor: a JWK entry with a non-empty `"x5c"` field.
3. Every `x5c` entry must be a parseable, standard base64-encoded DER X.509 certificate.

A network bundle that fails validation triggers the next fallback step.