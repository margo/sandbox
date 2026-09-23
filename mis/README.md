# MIS CLI

`mis-cli` is a command-line tool for managing SPIFFE Verifiable Identity Documents (SVIDs) and running the MIS REST API server.

MIS is an implementation of the Margo Identity and Authorization Framework (MIAF) MIS role. It issues X.509-SVIDs for a configured Trust Domain and publishes the Trust Domain discovery document and SPIFFE Trust Bundle over HTTPS. The normative HTTPS contract is the [Margo Trust Bundle API](https://docs.margo.org/specification/identity/trust-bundle-api-1.0.0-rc.3).

---

## Table of Contents

- [Building MIS](#building-mis)
- [Implementation Structure](#implementation-structure)
- [MIAF Implementation](#miaf-implementation)
- [Normative MIS API](#normative-mis-api)
- [Configuration](#configuration)
  - [Configuration Fields](#configuration-fields)
  - [Generating Configuration with confbuilder.sh](#generating-configuration-with-confbuildersh)
- [PKI Setup](#pki-setup)
  - [Generating Certificates with pki_gen.sh](#generating-certificates-with-pki_gensh)
- [Commands](#commands)
  - [start](#start)
  - [mint x509](#mint-x509)
- [Examples](#examples)
- [Output Files](#output-files)

## Implementation Structure

The `mis` directory is organized around the CLI, the two server surfaces, and the shared identity types:

```text
mis/
|-- main.go                         CLI entry point
|-- cli/                             Cobra commands: start and mint x509
|-- https/                           HTTPS server for the normative MIS API
|   |-- server.go                    Routes, TLS, caching, and HTTP responses
|   `-- operations/                  Discovery and Trust Bundle generation
|-- unix/                            Local HTTP server for SVID minting
|   |-- server.go                    Unix socket endpoint registration
|   |-- client/                      CLI client for the minting endpoint
|   `-- operations/                  X.509-SVID key and certificate creation
|-- pkg/
|   |-- conf/                        Configuration loading and validation
|   |-- types/                       Shared request, response, and service types
|   |-- helpers/                     Certificate and HTTP response helpers
|   `-- standard/generatedCode/      OpenAPI-generated normative API models/client
`-- certs/                           Certificate documentation and local PKI assets
```

When `mis start` runs, it starts both servers in parallel:

1. `https.MisRestAPI` listens on the configured HTTPS address and exposes the discovery and Trust Bundle endpoints.
2. `unix.MintRestAPI` listens on `/tmp/mint.sock` with mode `0600` and exposes the local minting endpoint.

The Unix socket is an implementation detail for local provisioning. It is not part of the normative Trust Bundle API and is not exposed as a network API.

## MIAF Implementation

[MIAF](https://docs.margo.org/specification/identity/identity-framework) defines the MIS as an identity-authority role within one Trust Domain. The role is responsible for issuing X.509-SVIDs, publishing trust material, and enforcing the MIAF identity and SVID profile rules. MIS implements that role as follows:

| MIAF responsibility | MIS implementation |
| --- | --- |
| Identify one Trust Domain | `trustDomain` is loaded from configuration and used for SPIFFE ID and bundle operations. |
| Issue X.509-SVIDs | `mis mint x509` sends a request to the local Unix socket; `unix/operations` generates an ECDSA P-256 key and signs an X.509 certificate with the configured CA. |
| Publish the Trust Bundle | `https/operations` loads the configured CA certificate and creates a SPIFFE bundle for the configured Trust Domain. |
| Publish discovery metadata | `GET /.well-known/margo` returns the Trust Domain and an absolute HTTPS `trustBundleUri`. |
| Support Trust Bundle retrieval | The configured `trustBundleURI` route returns the SPIFFE bundle as JSON. |

MIS reuses SPIFFE primitives through `go-spiffe`: SPIFFE IDs identify issued identities, X.509-SVIDs carry the SPIFFE ID in a URI SAN, and the published bundle contains the X.509 trust anchor for the Trust Domain. MIAF identity profiles require Margo identities to use a path beginning with `/margo/`; callers should therefore supply SPIFFE IDs such as `spiffe://margo.org/margo/...` when issuing MIAF identities.

Issuance is intentionally separated from the normative retrieval API. MIAF defines the MIS responsibilities and the Trust Bundle wire contract, but does not require a particular enrollment or issuance API. This implementation uses a local Unix socket to keep the minting operation off the network.

## Normative MIS API

MIS implements the read-only HTTPS endpoints defined by the [Margo Trust Bundle API 1.0.0-rc.3](https://docs.margo.org/specification/identity/trust-bundle-api-1.0.0-rc.3):

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/.well-known/margo` | Optional discovery document for one Trust Domain. |
| `GET` | `/<trustBundleURI>` | SPIFFE Trust Bundle for the configured Trust Domain. The default is `/.well-known/spiffe/bundle.json`. |

The discovery response has this shape:

```json
{
  "trustDomain": "margo.org",
  "trustBundleUri": "https://mis.margo.org/.well-known/spiffe/bundle.json"
}
```

The `trustBundleUri` is generated as an absolute HTTPS URL from the configured Trust Domain, listener port, and `trustBundleURI`. The Trust Bundle response is serialized in the SPIFFE bundle format and contains the configured CA as the X.509 authority, a sequence number, and the configured `refreshHint`.

Both successful JSON responses include an `ETag`. Clients can send `If-None-Match` and receive `304 Not Modified` when the document or bundle has not changed. Unsupported `Accept` values and unavailable resources are returned as `application/problem+json` using the generated Margo Problem Details model.

The endpoints use HTTPS. Initial trust for retrieving the bundle must be established externally, as described by the MIAF specification; the current server configures TLS certificates and a minimum TLS version of 1.3. The HTTPS CA is used to assemble the server certificate chain. Client-certificate authentication is not currently enabled by the HTTPS server.

The generated API models and client are in `pkg/standard/generatedCode`; they are generated from the checked-in standard API definition and should not be edited by hand.

---

## Building MIS

A `Makefile` is provided to build the binary and Docker image.

### Build the binary

```bash
make build
```

Produces a `mis` binary in the current directory. Cross-compilation is supported via environment variables:

```bash
make build GOOS=linux GOARCH=arm64
```

### Build a Docker image

```bash
make container
```

Builds and tags the image as `mis:<version>` and `mis:latest`. Override image name and tag:

```bash
make container IMAGE_NAME=myrepo/mis IMAGE_TAG=1.0.0
```

### Run locally (requires config and certs to be in place)

```bash
make run
```

This runs `go run main.go start --config ./configuration.json`.

### Clean build artifacts

```bash
make clean
```

### Available targets

| Target | Description |
|--------|-------------|
| `build` | Build the `mis` binary |
| `container` | Build Docker image |
| `run` | Run the server locally |
| `clean` | Remove build artifacts |
| `help` | Show usage summary |

---

## Configuration

MIS requires a JSON configuration file passed to the `start` command via `--config`.

### Example: `configuration.json`

```json
{
  "trustDomain": "margo.org",
  "refreshHint": 500,
  "trustBundleURI": ".well-known/spiffe/bundle.json",
  "log": {
    "level": "info"
  },
  "ca": {
    "cert": "./certs/ca.crt",
    "key": "./certs/ca.key"
  },
  "https": {
    "addr": ":18443",
    "ca": "./certs/https-ca.crt",
    "cert": "./certs/https-server.crt",
    "key": "./certs/https-server.key"
  }
}
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `trustDomain` | `string` | The SPIFFE trust domain for this deployment (e.g. `margo.org`). Used as the authority in SPIFFE IDs: `spiffe://<trustDomain>/...` |
| `refreshHint` | `integer` | Hint (in seconds) indicating how frequently clients should refresh the SPIFFE trust bundle from MIS. Must be a positive integer. |
| `trustBundleURI` | `string` | The URI path at which the SPIFFE trust bundle is served (e.g. `.well-known/spiffe/bundle.json`) |
| `log.level` | `string` | Logging verbosity. Accepted values: `debug`, `info`, `warn`, `error` |
| `ca.cert` | `string` | Path to the Minter CA certificate used to sign X.509 SVIDs |
| `ca.key` | `string` | Path to the Minter CA private key |
| `https.addr` | `string` | Address and port for the HTTPS REST API server (e.g. `:18443`, `:443`) |
| `https.ca` | `string` | Path to the HTTPS CA certificate used for mutual TLS client verification |
| `https.cert` | `string` | Path to the HTTPS server certificate |
| `https.key` | `string` | Path to the HTTPS server private key |

---

### Generating Configuration with `confbuilder.sh`

A helper script is provided at `scripts/lib/mis/confbuilder.sh` to generate `configuration.json` without manually editing JSON.

It supports two modes:

#### Automated (uses built-in defaults)

```bash
./scripts/lib/mis/confbuilder.sh --automated
```

Writes `configuration.json` to the current directory using default values.

#### Interactive (prompts for each field)

```bash
./scripts/lib/mis/confbuilder.sh --interactive
```

Walks through each configuration field with a prompt. Press **Enter** to accept the shown default.

```
  Example: margo.org, example.org, mycompany.io
  Trust Domain [default: margo.org]: _
```

The generated file is written to `$(pwd)/configuration.json`.

---

## PKI Setup

MIS requires two sets of certificates:

| Certificate Set | Purpose |
|----------------|---------|
| **Minter CA** (`ca.crt`, `ca.key`) | Signs X.509 SVIDs issued by `mis mint x509` |
| **HTTPS CA + Server cert** (`https-ca.crt`, `https-server.crt`, `https-server.key`) | Secures the REST API HTTPS server |

### Generating Certificates with `pki_gen.sh`

A PKI generator script is provided at `scripts/lib/mis/pki_gen.sh`. It generates all required certificates in a single run.

**Prerequisites:** `openssl` must be installed and available in `$PATH`.

#### Automated mode

```bash
./scripts/lib/mis/pki_gen.sh --automated
```

Uses built-in defaults to generate all certificates without prompts.

#### Interactive mode

```bash
./scripts/lib/mis/pki_gen.sh --interactive
```

Prompts for certificate fields (CN, Country, Organization, DNS SAN, validity periods, etc.) with defaults pre-filled.

#### Generated files

All files are written to `./certs/`:

| File | Description |
|------|-------------|
| `certs/https-ca.key` | HTTPS CA private key |
| `certs/https-ca.crt` | HTTPS CA self-signed certificate (10 years) |
| `certs/ca.key` | Minter CA private key |
| `certs/ca.crt` | Minter CA self-signed certificate (10 years) |
| `certs/https-server.key` | HTTPS server private key |
| `certs/https-server.crt` | HTTPS server certificate signed by HTTPS CA (1 year) |

The script also verifies the generated chain and prints a summary on completion.

> ⚠️ Private keys are written with `600` permissions and the `certs/` directory with `700`. Keep these files secure.

---

## Full Deployment Quickstart

```bash
# 1. Generate PKI certificates
./scripts/lib/mis/pki_gen.sh --automated

# 2. Generate configuration
./scripts/lib/mis/confbuilder.sh --automated

# 3. Build the binary
make build

# 4. Start the server
./mis start --config ./configuration.json
```

Or using Docker:

```bash
# 3. Build container
make container

# 4. Run container (mount certs and config)
docker run \
  -v $(pwd)/certs:/certs \
  -v $(pwd)/configuration.json:/configuration.json \
  -p 18443:18443 \
  mis:latest start --config /configuration.json
```

---

## Commands

### `start`

Starts the MIS REST API server and the SVID Mint server.

```bash
mis start --config <path-to-config>
```

**Flags:**

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--config` | `-c` | ✅ | Path to the configuration file |

**Graceful shutdown:** Send `SIGINT` or `SIGTERM` to stop both servers cleanly.

---

### `mint x509`

Mints an X.509 SVID and writes the certificate and private key to disk.

```bash
mis mint x509 --spiffeID <spiffe-id> [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--spiffeID` | ✅ | — | SPIFFE ID to embed in the SVID. Format: `spiffe://<trust-domain>/<path>` |
| `--dns` | ❌ | `[]` | DNS SAN to include. Repeatable for multiple entries |
| `--ttl` | ❌ | `86400` | Validity duration in seconds (24 hours) |
| `--outputDir` | ❌ | Current working directory | Directory to write output files |

**Validation rules:**
- `--spiffeID` must use the `spiffe://` scheme, include a non-empty trust domain and path
- `--ttl` must be a positive integer
- `--outputDir` must exist and be writable
- `--dns` values must be non-empty strings

---

## Examples

```bash
# Start the server
mis start --config /etc/mis/config.json

# Mint an X.509 SVID with defaults
mis mint x509 \
  --spiffeID spiffe://example.org/myservice

# Mint with DNS SANs, custom TTL, and output directory
mis mint x509 \
  --spiffeID spiffe://example.org/myservice \
  --dns myservice.example.com \
  --dns myservice-internal.example.com \
  --ttl 3600 \
  --outputDir /tmp/svids
```

---

## Output Files

The `mint x509` command writes two files to `--outputDir`:

| File | Permissions | Description |
|------|-------------|-------------|
| `payload-cert.pem` | `0644` | Generated X.509 SVID certificate |
| `payload-key.pem` | `0400` | Corresponding private key |

> ⚠️ Existing files will be **overwritten** without warning.

---

## Help

```bash
mis --help
mis start --help
mis mint x509 --help
```