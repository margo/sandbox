
# MIS Client Library

## Purpose

`client` is a Go package that provides a thin, TLS-aware wrapper around the
auto-generated Margo Identity Service (MIS) HTTP client. It simplifies
connecting to the MIS endpoint by handling root CA trust configuration and
exposing clean, context-aware methods for fetching the discovery document and
SPIFFE trust bundle.

---

## Functions

### `New(endpoint string, rootCAPEM []byte) (*MISClient, error)`

Creates a new `MISClient` configured with mutual TLS trust using the provided
root CA PEM bytes. The endpoint should be a full base URL
(e.g. `https://mis.margo.org:8443`). Returns an error if the CA certificate
cannot be parsed or the underlying client fails to initialize.

---

### `GetDiscoveryDocument(ctx context.Context, etag string) (*GetWellKnownMargoResponse, error)`

Calls `GET /.well-known/margo` to retrieve the MIS discovery document.
Supports conditional GETs via an `If-None-Match` header — pass a non-empty
`etag` to enable it. When the server responds with `304 Not Modified`,
`JSON200` will be `nil`; callers should reuse their cached document.

---

### `GetTrustBundle(ctx context.Context, trustBundleURL string, etag string) (*GetWellKnownSpiffeBundleJsonResponse, error)`

Fetches the SPIFFE trust bundle. Accepts an optional `trustBundleURL` override
(e.g. sourced from the discovery document's `trustBundleUri` field); when
empty, it defaults to `/.well-known/spiffe/bundle.json` on the configured
server. Supports conditional GETs via `etag`. On `304 Not Modified`, `JSON200`
is `nil` and callers should reuse their cached bundle.

---

## Usage

### Installation

```go
import "github.com/margo/sandbox/mis/shared-lib/mis/client"
```

---

### Example 1 — Create a client

```go
rootCA, err := os.ReadFile("./certs/https-ca.crt")
if err != nil {
    log.Fatal(err)
}

misClient, err := client.New("https://mis.margo.org:8443", rootCA)
if err != nil {
    log.Fatal(err)
}
```

---

### Example 2 — Fetch the discovery document (with caching)

```go
var cachedETag string
var cachedDoc *generatedCode.GetWellKnownMargoResponse

rsp, err := misClient.GetDiscoveryDocument(ctx, cachedETag)
if err != nil {
    log.Fatal(err)
}

if rsp.HTTPResponse.StatusCode == http.StatusNotModified {
    // Server confirms document unchanged — reuse cachedDoc
} else {
    cachedDoc = rsp
    cachedETag = rsp.HTTPResponse.Header.Get("ETag")
}
```

---

### Example 3 — Fetch the SPIFFE trust bundle

```go
// Using default path
rsp, err := misClient.GetTrustBundle(ctx, "", "")
if err != nil {
    log.Fatal(err)
}

if rsp.HTTPResponse.StatusCode == http.StatusOK {
    bundle := rsp.JSON200
    fmt.Printf("Trust bundle keys: %+v\n", bundle)
}

// Using a custom URL from the discovery document
rsp, err = misClient.GetTrustBundle(ctx, "https://mis.margo.org:8443/.well-known/spiffe/bundle.json", "")
```
