# `trustbundle` Package

## Purpose

The `trustbundle` package retrieves `[SPIFFE](https://spiffe.io/)` trust bundles from a **Margo Identity Service (MIS)** endpoint. It implements a three-tier resilience strategy:

1. **Discovery** — fetches the canonical bundle URL from `/.well-known/margo`
2. **URI Fallback** — uses a pre-configured URI path if discovery fails
3. **Static Fallback** — returns an operator-supplied bundle if both network sources fail

It also supports **ETag-based caching** (`HTTP 304 Not Modified`) to avoid redundant transfers.

---

## Function Reference

### `New(misEndpoint, misRootCA, trustBundleURI, staticBundle, trustDomain) (Getter, error)`

Factory function. Validates all inputs and returns a `Getter` implementation.

| Parameter | Description |
|---|---|
| `misEndpoint` | HTTPS URL of the MIS server (e.g. `https://mis.example.com`) |
| `misRootCA` | PEM-encoded X.509 CA certificate for TLS verification |
| `trustBundleURI` | Optional URI path fallback (e.g. `/.well-known/spiffe/bundle.json`) |
| `staticBundle` | Optional JWKS JSON bytes used as last-resort fallback |
| `trustDomain` | Trust domain string (e.g. `margo.org`) used with static/URI fallback |

Returns an error if any provided value fails validation.

---

### `GetTrustBundle(ctx, ietag) (trustDomain, bundle, etag, error)`

Core retrieval method on the `Getter` interface. Executes the three-tier strategy:

- **200 OK** → returns `(trustDomain, bundleBytes, etag, nil)`
- **304 Not Modified** → returns `(trustDomain, nil, "", ErrNotModified)` — caller should keep its cached bundle
- **All sources fail** → returns a wrapped error chain describing each failure

---

### Internal Helpers

| Function | Description |
|---|---|
| `fallbackToURIOrStatic` | Orchestrates URI fallback then static fallback when discovery fails |
| `staticFallback` | Returns the static bundle; errors if either `staticBundle` or `trustDomain` is missing |
| `fetchBundleFromURL` | Calls the MIS client, handles 200/304/other status codes, marshals the response |
| `validateMISEndpoint` | Ensures the endpoint is a non-empty, parseable HTTPS URL with a host |
| `validateRootCA` | Ensures PEM bytes contain at least one valid X.509 certificate |
| `validateTrustBundleURI` | Ensures the URI is a path (not a full URL) starting with `/` |
| `validateSPIFFEBundle` | Lightweight check that the bytes are a JWKS JSON object with a `keys` array |

---

## Usage

### Installation

```bash
go get github.com/margo/sandbox/shared-lib/mis/trustbundle
```

---

### Basic Usage — Discovery Only

```go
package main

import (
    "context"
    "errors"
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

    getter, err := trustbundle.New(
        "https://mis.example.com", // MIS endpoint
        rootCA,                    // PEM root CA
        "",                        // no URI fallback
        nil,                       // no static fallback
        "",                        // no static trust domain
    )
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

### Full Resilience — URI and Static Fallbacks

```go
staticBundle, _ := os.ReadFile("/etc/mis/fallback-bundle.jwks")
rootCA, _        := os.ReadFile("/etc/mis/ca.pem")

getter, err := trustbundle.New(
    "https://mis.example.com",
    rootCA,
    "/.well-known/spiffe/bundle.json", // URI fallback path
    staticBundle,                       // static fallback bytes
    "margo.org",                        // trust domain for fallbacks
)
if err != nil {
    log.Fatalf("config error: %v", err)
}

_, bundle, _, err := getter.GetTrustBundle(context.Background(), "")
if err != nil {
    log.Fatalf("all sources exhausted: %v", err)
}
```

---

## Error Handling Reference

| Scenario | Returned error |
|---|---|
| Server returns HTTP 304 | `trustbundle.ErrNotModified` |
| Invalid config in `New()` | Descriptive validation error |
| All three sources fail | Wrapped error chain (use `errors.Is` / `errors.As`) |