# MIS CLI

`mis-cli` is a command-line tool for managing SPIFFE Verifiable Identity Documents (SVIDs) and running the MIS REST API server.

---

## Table of Contents

- [Building MIS](#building-mis)
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
  "trustDomain": "northstarida.com",
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
| `trustDomain` | `string` | The SPIFFE trust domain for this deployment (e.g. `northstarida.com`). Used as the authority in SPIFFE IDs: `spiffe://<trustDomain>/...` |
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
  Example: northstarida.com, example.org, mycompany.io
  Trust Domain [default: northstarida.com]: _
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
| `certs/server.key` | HTTPS server private key |
| `certs/server.crt` | HTTPS server certificate signed by HTTPS CA (1 year) |

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
