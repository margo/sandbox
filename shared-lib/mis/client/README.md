# MIS Client Library

## Purpose

`client` is a Go package that provides a thin, TLS-aware client for fetching the
Margo Identity Service (MIS) discovery document and SPIFFE trust bundle. The
caller supplies full, absolute HTTPS URLs for each request. The trust mode is
selected per request from the URL host, so the same client can reach servers
whose addresses change over time.

---

## Types

### `TrustConfig`

Holds the trust material used to authenticate MIS HTTPS servers. At least one
of `RootCAPEM` or `SPKIPins` must be non-empty. The material required for a
given request is determined by the URL host:

- **IP address host:** `SPKIPins` is required. At least one certificate in the
  server's chain must match a pin. Hostname verification is not performed,
  because the pin establishes identity.
- **DNS name host:** `RootCAPEM` is required. The chain is validated against
  these anchors with RFC 5280 path validation and RFC 9525 name checks, handled
  by the standard TLS stack.

| Field | Type | Description |
|---|---|---|
| `RootCAPEM` | `[]byte` | One or more PEM-encoded CA certificates. Used for DNS-named URLs. |
| `SPKIPins` | `[]string` | Base64-encoded SHA-256 SPKI digests in the form `sha256/<base64>` per RFC 7469 §2.4. Used for IP-addressed URLs. |

---

## Functions

### `New(trust TrustConfig) (*MISClient, error)`

Creates a new `MISClient` using the provided `TrustConfig`. The trust material
is validated up front. The trust mode for each request is selected later, from
the URL passed to `GetDiscoveryDocument` or `GetTrustBundle`.

Returns an error if neither `RootCAPEM` nor `SPKIPins` is provided, if the root
CA PEM cannot be parsed, or if any SPKI pin is malformed.

---

### `GetDiscoveryDocument(ctx context.Context, discoveryURL string, etag string) (*GetWellKnownMargoResponse, error)`

Calls the supplied discovery document URL and returns the parsed response.

- `discoveryURL` is a full absolute HTTPS URL, for example
  `https://192.0.2.10/.well-known/margo`.
- Pass a non-empty `etag` to send an `If-None-Match` header (conditional GET).
  Pass `""` to skip it.
- On `304 Not Modified`, `JSON200` is `nil`. The caller should reuse its cached
  document.
- On `404 Not Found`, the problem detail is returned in
  `ApplicationproblemJSON404`.

---

### `GetTrustBundle(ctx context.Context, trustBundleURL string, etag string) (*GetWellKnownSpiffeBundleJsonResponse, error)`

Calls the supplied SPIFFE bundle URL and returns the parsed response.

- `trustBundleURL` is a full absolute HTTPS URL, typically the `trustBundleUri`
  value from the discovery document or from operator configuration.
- Pass a non-empty `etag` to send an `If-None-Match` header (conditional GET).
  Pass `""` to skip it.
- On `304 Not Modified`, `JSON200` is `nil`. The caller should reuse its cached
  bundle.
- On `404 Not Found`, the problem detail is returned in
  `ApplicationproblemJSON404`.

---

## Validation Rules

Each request is checked before any network call is made:

- The URL must parse and use the `https` scheme.
- The URL host must not be empty.
- An IP-addressed URL requires `SPKIPins`.
- A DNS-named URL requires `RootCAPEM`.

---

## Usage

### Installation

```go
import "github.com/margo/sandbox/shared-lib/mis/client"
```

---

### Example 1: Create a client with a root CA certificate (DNS-named URLs)

```go
rootCA, err := os.ReadFile("./certs/https-ca.crt")
if err != nil {
    log.Fatal(err)
}

misClient, err := client.New(client.TrustConfig{
    RootCAPEM: rootCA,
})
if err != nil {
    log.Fatal(err)
}
```

---

### Example 2: Create a client with SPKI pins (IP-addressed URLs)

```go
misClient, err := client.New(client.TrustConfig{
    SPKIPins: []string{
        "sha256/YLh1dUR9y6Kja30RrAn7JKnbQG/uEtLMkBgFF2Fuihg=",
    },
})
if err != nil {
    log.Fatal(err)
}
```

---

### Example 3: Fetch the discovery document (with caching)

```go
var cachedETag string
var cachedDoc *generatedCode.GetWellKnownMargoResponse

rsp, err := misClient.GetDiscoveryDocument(ctx, "https://192.0.2.10/.well-known/margo", cachedETag)
if err != nil {
    log.Fatal(err)
}

if rsp.HTTPResponse.StatusCode == http.StatusNotModified {
    // Server confirms document unchanged. Reuse cachedDoc.
} else {
    cachedDoc = rsp
    cachedETag = rsp.HTTPResponse.Header.Get("ETag")
}
```

---

### Example 4: Fetch the SPIFFE trust bundle

```go
rsp, err := misClient.GetTrustBundle(ctx, "https://192.0.2.10/.well-known/spiffe/bundle.json", "")
if err != nil {
    log.Fatal(err)
}

if rsp.HTTPResponse.StatusCode == http.StatusOK {
    bundle := rsp.JSON200
    fmt.Printf("Trust bundle: %+v\n", bundle)
}
```

---

### Example 5: Use the trustBundleUri from the discovery document

```go
discovery, err := misClient.GetDiscoveryDocument(ctx, "https://mis.factory.example/.well-known/margo", "")
if err != nil {
    log.Fatal(err)
}

if discovery.JSON200 != nil {
    rsp, err := misClient.GetTrustBundle(ctx, discovery.JSON200.TrustBundleUri, "")
    if err != nil {
        log.Fatal(err)
    }
    _ = rsp
}
```