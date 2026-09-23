# `mtls` Package Documentation

The `mtls` package provides mutual TLS (mTLS) configuration helpers with full [SPIFFE](https://spiffe.io/) X.509-SVID peer authentication, replacing Go's default hostname/DNS verification with a four-rule SPIFFE-compliant verification pipeline ([SVID Validation](https://docs.margo.org/specification/identity/svids#x509-svid-validation)).

---

## Purpose

This package enables services to establish **mutually authenticated TLS connections** using SPIFFE identities (URI SANs) instead of DNS hostnames according to MIAF. It is designed for zero-trust service meshes where every workload presents a cryptographically verifiable SPIFFE ID.

**Key guarantees enforced on every connection:**

| Rule | Description |
|------|-------------|
| **Rule 1** | Peer's trust domain must match the verifier's own trust domain |
| **Rule 2** | Peer's certificate chain must validate against the SPIFFE trust bundle |
| **Rule 3** | Peer's SVID must be within its validity period |
| **Rule 4** | Peer's SVID leaf must satisfy SPIFFE leaf constraints |
| **Allow-list** | Peer's SPIFFE ID must appear in the configured allow list |

---

## Types

### `VerifierConfig`

Holds the runtime configuration for SPIFFE peer verification. All fields are **functions** (not values) to support dynamic credential rotation without restarting the server or client.

```go
type VerifierConfig struct {
    // Returns the verifier's own SPIFFE trust domain (e.g. "example.org").
    // Only SVIDs from this domain are accepted (Rule 1).
    GetOwnTrustDomain func() string

    // Returns the SPIFFE trust bundle in JWK Set (JSON) format.
    // Used to validate the peer's certificate chain (Rule 2).
    GetTrustBundleBytes func() []byte

    // Returns the list of permitted SPIFFE IDs.
    // An empty list rejects all connections.
    GetClientAllowList func() []string
}
```

---

## Functions

### `NewMTLSClientConfig`

```go
func NewMTLSClientConfig(clientCert tls.Certificate, cfg VerifierConfig) (*tls.Config, error)
```

Returns a `*tls.Config` for a **TLS client** performing mutual authentication.

- Sets `MinVersion` to TLS 1.3
- Disables Go's built-in hostname verification (`InsecureSkipVerify: true`) — intentional, as SPIFFE identity lives in the URI SAN, not DNS SAN or CN
- Attaches a `VerifyConnection` hook that enforces all four SPIFFE rules plus the allow-list check
- The caller must supply the client's own `tls.Certificate` so the server can authenticate the client

---

### `NewMTLSServerConfig`

```go
func NewMTLSServerConfig(serverCert tls.Certificate, cfg VerifierConfig) (*tls.Config, error)
```

Returns a `*tls.Config` for a **TLS server** performing mutual authentication.

- Sets `MinVersion` to TLS 1.3
- Sets `ClientAuth: tls.RequireAnyClientCert` — the client *must* present a certificate during the handshake
- Attaches a `VerifyConnection` hook for full SPIFFE validation
- The caller must supply the server's own `tls.Certificate` so the client can authenticate the server

---

### `buildVerifyConnection` *(internal)*

```go
func buildVerifyConnection(cfg VerifierConfig, principal string) func(tls.ConnectionState) error
```

Constructs the `VerifyConnection` callback shared by both client and server configs. Executes the full verification pipeline in order:

1. Validates that trust domain and trust bundle are available
2. Checks that the peer presented at least one certificate
3. **Rule 1:** Extracts and compares the peer's SPIFFE trust domain
4. **Rule 2:** Validates the certificate chain against the trust bundle (with or without intermediates)
5. **Rule 3/4:** Validates SVID validity period and leaf constraints
6. **Allow-list:** Confirms the peer's SPIFFE ID is in the configured allow list

---

### `verifyChainAgainstBundle` *(internal)*

```go
func verifyChainAgainstBundle(leaf *x509.Certificate, rawIntermediates [][]byte, trustBundle []byte) error
```

Validates the peer's certificate chain against the SPIFFE trust bundle. Handles two cases:
- **No intermediates:** Validates the leaf directly against the trust bundle
- **With intermediates:** Delegates to `verifyWithIntermediates`

---

### `verifyWithIntermediates` *(internal)*

```go
func verifyWithIntermediates(leaf *x509.Certificate, intermediates *x509.CertPool, trustBundle []byte) error
```

Attempts chain validation when intermediate certificates are present:
1. First tries direct leaf validation against the trust bundle
2. Falls back to building a root pool from the JWK trust bundle and running `x509.Certificate.Verify` with the intermediate pool

---

## Usage Examples

### As a Library

Import the package:

```go
import "github.com/margo/sandbox/shared-lib/mis/mtls"
```

---

### Example 1: Configuring an mTLS Server

```go
package main

import (
    "crypto/tls"
    "net/http"

    "github.com/margo/sandbox/shared-lib/mis/mtls"
)

func main() {
    // Load the server's own SPIFFE SVID certificate and key.
    serverCert, err := tls.LoadX509KeyPair("server.crt", "server.key")
    if err != nil {
        panic(err)
    }

    cfg := mtls.VerifierConfig{
        GetOwnTrustDomain: func() string {
            return "example.org"
        },
        GetTrustBundleBytes: func() []byte {
            bundle, _ := os.ReadFile("trust-bundle.json") // JWK Set format
            return bundle
        },
        GetClientAllowList: func() []string {
            return []string{
                "spiffe://example.org/ns/default/sa/my-client",
            }
        },
    }

    tlsConfig, err := mtls.NewMTLSServerConfig(serverCert, cfg)
    if err != nil {
        panic(err)
    }

    server := &http.Server{
        Addr:      ":8443",
        TLSConfig: tlsConfig,
    }

    // Certificates are already in TLSConfig; pass empty strings to ListenAndServeTLS.
    server.ListenAndServeTLS("", "")
}
```

---

### Example 2: Configuring an mTLS Client

```go
package main

import (
    "crypto/tls"
    "net/http"

    "github.com/margo/sandbox/shared-lib/mis/mtls"
)

func main() {
    // Load the client's own SPIFFE SVID certificate and key.
    clientCert, err := tls.LoadX509KeyPair("client.crt", "client.key")
    if err != nil {
        panic(err)
    }

    cfg := mtls.VerifierConfig{
        GetOwnTrustDomain: func() string {
            return "example.org"
        },
        GetTrustBundleBytes: func() []byte {
            bundle, _ := os.ReadFile("trust-bundle.json")
            return bundle
        },
        GetClientAllowList: func() []string {
            return []string{
                "spiffe://example.org/ns/default/sa/my-server",
            }
        },
    }

    tlsConfig, err := mtls.NewMTLSClientConfig(clientCert, cfg)
    if err != nil {
        panic(err)
    }

    httpClient := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }

    resp, err := httpClient.Get("https://my-server.example.org:8443/api/resource")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
}
```

---

### Example 3: Dynamic Credential Rotation

Because `VerifierConfig` fields are **functions**, credentials can be rotated at runtime without rebuilding the TLS config:

```go
var (
    mu          sync.RWMutex
    trustBundle []byte
    allowList   []string
)

cfg := mtls.VerifierConfig{
    GetOwnTrustDomain: func() string {
        return "example.org"
    },
    GetTrustBundleBytes: func() []byte {
        mu.RLock()
        defer mu.RUnlock()
        return trustBundle
    },
    GetClientAllowList: func() []string {
        mu.RLock()
        defer mu.RUnlock()
        return allowList
    },
}

// In a background goroutine, refresh credentials periodically:
go func() {
    for range time.Tick(30 * time.Second) {
        newBundle, _ := fetchLatestTrustBundle()
        newList, _ := fetchLatestAllowList()
        mu.Lock()
        trustBundle = newBundle
        allowList = newList
        mu.Unlock()
    }
}()
```

---

## Security Notes

- `InsecureSkipVerify: true` in the client config is **intentional** and safe here — it only disables DNS hostname matching, which is replaced by SPIFFE URI SAN verification via `VerifyConnection`.
- An **empty allow list** is a hard deny-all — no connections will be accepted.
- TLS 1.3 is enforced as the minimum version; TLS 1.2 and below are rejected.