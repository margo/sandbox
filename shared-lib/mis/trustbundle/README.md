# `trustbundle` Package

## Purpose

The `trustbundle` package retrieves `[SPIFFE](https://spiffe.io/)` trust bundles from a **Margo Infrastructure Service (MIS)** endpoint. It implements a three-tier resilience strategy:

1. **Discovery** — fetches the canonical bundle URL and trust domain from `/.well-known/margo`
2. **URI Fallback** — uses a pre-configured URI path if discovery fails
3. **Static Fallback** — returns an operator-supplied bundle if both network sources fail

ETag-based caching (`HTTP 304 Not Modified`) is supported to avoid redundant transfers.

---

## API Reference

### `New(cfg TrustBundleGetterConfig) (Getter, error)`

Factory function. Validates all inputs and returns a `Getter` implementation.

```go
type TrustBundleGetterConfig struct {
    MISEndpoint    string
    MISRootCA      []byte
    MISTrustPins   []string
    TrustBundleURI string
    StaticBundle   []byte
    TrustDomain    string
}
```

| Field | Required | Description |
|---|---|---|
| `MISEndpoint` | ⚠️ | HTTPS URL of the MIS server (e.g. `https://mis.example.com`). Required unless using static bundle-only mode. |
| `MISRootCA` | ⚠️ | PEM-encoded X.509 CA certificate for TLS verification. Takes precedence over `MISTrustPins` when both are set. |
| `MISTrustPins` | ⚠️ | Base64-encoded SHA-256 SPKI pins (RFC 7469 §2.4). Used when `MISRootCA` is absent. |
| `TrustBundleURI` | ❌ | Optional URI path fallback (e.g. `/.well-known/spiffe/bundle.json`). Must be a path, not a full URL. |
| `StaticBundle` | ❌ | Optional JWKS JSON bytes used as last-resort fallback. |
| `TrustDomain` | ⚠️ | Required when `StaticBundle` is configured. |

**Valid configurations:**
- **Endpoint mode:** `MISEndpoint` + at least one trust anchor (`MISRootCA` or `MISTrustPins`)
- **Static bundle mode:** `StaticBundle` + `TrustDomain` (no endpoint required)
- Both modes can coexist; endpoint mode is attempted first with static as fallback

> When both `MISRootCA` and `MISTrustPins` are provided, `MISRootCA` takes precedence and `MISTrustPins` is silently ignored.

Returns a descriptive validation error if any input is invalid.

---

### `GetTrustBundle(ctx context.Context, ietag string) (trustDomain string, bundle []byte, etag string, err error)`

Core retrieval method on the `Getter` interface. Executes the three-tier strategy:

| Result | Returns |
|---|---|
| **HTTP 200** | `(trustDomain, bundleBytes, etag, nil)` |
| **HTTP 304 Not Modified** | `(trustDomain, nil, "", ErrNotModified)` — caller should keep its cached bundle |
| **All sources fail** | `("", nil, "", wrappedError)` — error chain describes each failure |

**Retrieval strategy:**

1. **Discovery:** `GET /.well-known/margo` → extracts `trustBundleUri` and `trustDomain` → fetches bundle from discovered URL.
2. **URI Fallback:** If discovery fails and `TrustBundleURI` is set, fetches from `MISEndpoint + TrustBundleURI`.
3. **Static Fallback:** Returns `StaticBundle` and `TrustDomain` as configured.

> **Important:** HTTP 304 short-circuits the fallback chain. The static bundle is **not** attempted on 304, since the cached network bundle is authoritative.

---

### `ErrNotModified`

```go
var ErrNotModified = errors.New("trust bundle not modified")
```

Returned by `GetTrustBundle` when the server responds with HTTP 304, indicating the caller's cached bundle is still current.

---

## Usage Examples

### Root CA Only

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

    getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
        MISEndpoint: "https://mis.example.com",
        MISRootCA:   rootCA,
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

### Trust Pins Only (No Root CA)

```go
getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
    MISEndpoint:  "https://mis.example.com",
    MISTrustPins: []string{"sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
})
if err != nil {
    log.Fatalf("invalid config: %v", err)
}
```

---

### ETag Caching — Avoid Redundant Fetches

```go
var cachedBundle []byte
var cachedETag   string
var cachedDomain string

func refreshBundle(ctx context.Context, getter trustbundle.Getter) error {
    domain, bundle, etag, err := getter.GetTrustBundle(ctx, cachedETag)
    if err != nil {
        if errors.Is(err, trustbundle.ErrNotModified) {
            // Server confirmed our cache is still valid — nothing to do.
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

### Full Resilience — All Fallbacks Configured

```go
rootCA, _        := os.ReadFile("/etc/mis/ca.pem")
staticBundle, _  := os.ReadFile("/etc/mis/fallback-bundle.jwks")

getter, err := trustbundle.New(trustbundle.TrustBundleGetterConfig{
    MISEndpoint:    "https://mis.example.com",
    MISRootCA:      rootCA,
    TrustBundleURI: "/.well-known/spiffe/bundle.json",
    StaticBundle:   staticBundle,
    TrustDomain:    "margo.org",
})
if err != nil {
    log.Fatalf("config error: %v", err)
}

trustDomain, bundle, etag, err := getter.GetTrustBundle(context.Background(), "")
if err != nil {
    log.Fatalf("all sources exhausted: %v", err)
}
```

---

### Static Bundle Only (No MIS Endpoint)

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
| HTTP 304 from any network source | `trustbundle.ErrNotModified` |
| `MISEndpoint` set without a trust anchor, or vice versa | Validation error from `New()` |
| Neither endpoint mode nor static bundle configured | Validation error from `New()` |
| `StaticBundle` set without `TrustDomain` | Validation error from `New()` |
| `TrustBundleURI` is a full URL instead of a path | Validation error from `New()` |
| Bundle fails SPIFFE structural/crypto validation | Wrapped validation error |
| All three sources fail | Wrapped error chain — use `errors.Is` / `errors.As` |

---

## Bundle Validation

All bundles fetched from the network (and any `StaticBundle` supplied at construction) are validated against these rules:

1. Must be valid JSON containing a `"keys"` array (JWKS format).
2. Must contain at least one X.509 trust anchor — a JWK entry with a non-empty `"x5c"` field.
3. Every `x5c` entry must be a parseable, standard base64-encoded DER X.509 certificate.