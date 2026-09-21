# Margo Identity Service helpers

This directory contains Go helpers for components that participate in the
[Margo Identity and Authorization Framework (MIAF)](https://docs.margo.org/specification/identity/identity-framework).
MIAF uses SPIFFE identity primitives to give Margo's non-human components a
verifiable identity, authenticate them with mTLS, and authorize requests using
local policy.

The packages here are client-side building blocks for that model. They help a
Margo component load its X.509-SVID and private key, discover and retrieve its
Trust Domain's SPIFFE Trust Bundle from a Margo Identity Service (MIS), verify
peer SVIDs, and apply an allow-list of authorized SPIFFE IDs.

## What this package signifies

In MIAF terminology, an MIS is an identity-authority **role** within a Trust
Domain. The MIS issues SVIDs and publishes the discovery document and Trust
Bundle. The code in this directory is not itself an MIS implementation and
does not define an enrollment or SVID-issuance protocol. Instead, it provides
the helpers a WFM, WFM Client, or other MIAF-enabled component can use to:

1. obtain the MIS discovery document and Trust Bundle;
2. establish an mTLS connection using its own SVID;
3. validate a peer's certificate chain, SVID constraints, and SPIFFE ID; and
4. authorize the verified peer locally.

MIAF authentication and authorization are deliberately separate steps:

```text
SVID + Trust Bundle
				|
				v
	mTLS authentication ----> verified SPIFFE ID
																			|
																			v
												 local authorization policy
```

The peer's verified SPIFFE ID is the input to authorization. There is no
central authorization server or OAuth-style scope exchange in this flow.

## Package map

| Package | MIAF responsibility |
| --- | --- |
| [`client`](client/README.md) | TLS-aware HTTP client for the MIS discovery document and SPIFFE Trust Bundle endpoints. Supports `ETag`-based conditional requests. |
| [`trustbundle`](trustbundle/README.md) | Retrieves a Trust Bundle through discovery, a configured URI, or an operator-provided static fallback. Exposes `ErrNotModified` for cache reuse. |
| [`parser`](parser/config.go) | Loads file-backed MIAF configuration, parses SPIFFE IDs and certificates, and creates `tls.Certificate` values from PEM or DER inputs. |
| [`validators`](validators/svid.go) | Validates trust domains, SPIFFE IDs, X.509-SVID leaf constraints, private keys, CA certificates, and JWKS-formatted SPIFFE Trust Bundles. |
| [`mtls`](mtls/README.md) | Builds TLS client and server configurations with TLS 1.3, peer SVID verification, Trust Bundle validation, Trust Domain checks, and local SPIFFE-ID allow-lists. |
| `encoders` | Converts parsed certificates to PEM and builds `x509.CertPool` values for TLS and certificate verification. |

## Typical component flow

An application using these helpers generally follows this sequence:

1. Read operator configuration with [`parser.ParseMIAFConfig`](parser/config.go).
2. Load the component's certificate and key with
	 [`parser.CertificateFromBytes`](parser/cert.go).
3. Retrieve and cache the Trust Bundle with [`trustbundle.New`](trustbundle/getTrustBundle.go).
	 Discovery uses `GET /.well-known/margo`; the bundle is then fetched from the
	 URI advertised by the discovery document. A configured URI and static bundle
	 can be used as fallbacks.
4. Create an mTLS configuration with
	 [`mtls.NewMTLSClientConfig`](mtls/client.go) or
	 [`mtls.NewMTLSServerConfig`](mtls/server.go).
5. Let the TLS verifier authenticate the peer. It checks the peer's SPIFFE
	 Trust Domain, certificate chain, validity period, X.509-SVID leaf
	 constraints, and configured authorization allow-list.

The Trust Bundle is a SPIFFE JWKS document containing X.509 trust anchors. It
is not the component's own SVID and should not be treated as a bearer token.
The component's SVID identifies it; the Trust Bundle tells it which issuers it
trusts.

## Important boundaries

- **Trust Domain:** peer identities are expected to belong to the verifier's
	configured Trust Domain. Cross-Trust-Domain federation is not provided by
	these helpers.
- **Authentication:** mTLS proves possession of the private key associated with
	the presented X.509-SVID and validates its chain against the Trust Bundle.
- **Authorization:** the verifier makes the decision locally from the verified
	SPIFFE ID and its configured allow-list. Operators must keep that policy
	current.
- **Credential lifecycle:** this package loads and validates supplied material;
	it does not provision, rotate, or issue SVIDs. Applications are responsible
	for refreshing credentials and rebuilding or updating runtime configuration
	as required.

## Example import

```go
import (
		"github.com/margo/sandbox/shared-lib/mis/mtls"
		"github.com/margo/sandbox/shared-lib/mis/parser"
		"github.com/margo/sandbox/shared-lib/mis/trustbundle"
)
```

See the README in each subpackage for its API details and examples.
