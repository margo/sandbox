# Margo Identity Service (MIS) — Scripts Documentation

## 1. `confbuilder.sh` — Configuration Builder

### Purpose

Generates a `configuration.json` file for the Margo Identity Service. Supports two modes: **interactive** (prompts for each value with defaults) and **automated** (writes defaults without prompting). The output file is always written to the current working directory.

### Prerequisites

- Bash 4+
- Write permission in the current directory

### Usage

```bash
./confbuilder.sh [--interactive | --automated]
```

| Flag            | Description                                          |
|-----------------|------------------------------------------------------|
| `--interactive` | Prompts for each configuration value with defaults   |
| `--automated`   | Writes `configuration.json` using built-in defaults  |

### Configuration Fields & Defaults

| Field             | Default                              | Description                              |
|-------------------|--------------------------------------|------------------------------------------|
| `trustDomain`     | `margo.org`                          | SPIFFE trust domain                      |
| `refreshHint`     | `500`                                | Bundle refresh interval (seconds)        |
| `trustBundleURI`  | `.well-known/spiffe/bundle.json`     | URI path for the trust bundle            |
| `log.level`       | `info`                               | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `ca.cert`         | `./ca.crt`                           | Path to the Minter CA certificate        |
| `ca.key`          | `./ca.key`                           | Path to the Minter CA private key        |
| `https.addr`      | `:8443`                              | HTTPS listen address                     |
| `https.ca`        | `./https-ca.crt`                     | Path to the HTTPS CA certificate         |
| `https.cert`      | `./https-server.crt`                 | Path to the HTTPS server certificate     |
| `https.key`       | `./https-server.key`                 | Path to the HTTPS server private key     |

### Output

A `configuration.json` file in the current working directory:

```json
{
  "trustDomain": "margo.org",
  "refreshHint": 500,
  "trustBundleURI": ".well-known/spiffe/bundle.json",
  "log": {
    "level": "info"
  },
  "ca": {
    "cert": "./ca.crt",
    "key": "./ca.key"
  },
  "https": {
    "addr": ":8443",
    "ca": "./https-ca.crt",
    "cert": "./https-server.crt",
    "key": "./https-server.key"
  }
}
```

### Examples

```bash
# Generate configuration.json using all defaults (no prompts)
./confbuilder.sh --automated

# Interactively configure each field (press Enter to accept defaults)
./confbuilder.sh --interactive

# Show usage
./confbuilder.sh
```

---

## 2. `pki_gen.sh` — PKI Generator

### Purpose

Generates a complete PKI (Public Key Infrastructure) for the Margo Identity Service, producing three artifacts:

| Artifact                  | Algorithm     | Purpose                                              |
|---------------------------|---------------|------------------------------------------------------|
| **HTTPS CA**              | RSA-4096      | Signs the HTTPS server certificate                   |
| **Minter CA**             | EC prime256v1 | Signs X.509 SVIDs (SPIFFE Verifiable Identity Docs)  |
| **HTTPS Server Cert**     | RSA-4096      | TLS certificate for the MIS HTTPS endpoint           |

All files are written to `./certs/`.

### Prerequisites

- Bash 4+
- `openssl` installed and available in `PATH`

### Usage

```bash
./pki_gen.sh [--interactive | --automated] [--dns <DNS_SAN>] [--minter-cn <CN>]
```

| Flag                    | Description                                                                 |
|-------------------------|-----------------------------------------------------------------------------|
| `--interactive`         | Prompts for all certificate fields                                           |
| `--automated`           | Uses built-in defaults (no prompts)                                          |
| `--dns <value>`         | Override the server DNS SAN (automated mode only)                            |
| `--minter-cn <value>`   | Override the Minter CA Common Name (automated mode only)                     |
| `--help`, `-h`          | Show usage information                                                       |

> If no mode flag is provided, the script interactively asks you to choose between modes.

### Default Values

| Field                  | Default                  |
|------------------------|--------------------------|
| Common Name (CN)       | `Capgemini`              |
| Country (C)            | `IN`                     |
| State (ST)             | `Haryana`                |
| Locality (L)           | `Gurugram`               |
| Organization (O)       | `Capgemini`              |
| Org Unit (OU)          | `Margo Sandbox Team`     |
| Email                  | `admin@capgemini.com`    |
| CA Validity            | `3650` days (10 years)   |
| Server Cert Validity   | `365` days (1 year)      |
| Server DNS SAN         | `mis.margo.org`          |
| Minter CA CN           | `margo.org`              |

### Generated Files

```
./certs/
├── https-ca.key          # HTTPS CA private key (RSA-4096, chmod 600)
├── https-ca.crt          # HTTPS CA self-signed certificate
├── ca.key                # Minter CA private key (EC prime256v1, chmod 600)
├── ca.crt                # Minter CA self-signed certificate
├── https-server.key      # Server private key (RSA-4096, chmod 600)
└── https-server.crt      # Server certificate signed by HTTPS CA
```

> Temporary files (`https-server.csr`, `https-server_ext.cnf`, `https-ca.srl`) are automatically removed after generation.

### Examples

```bash
# Automated mode with all defaults
./pki_gen.sh --automated

# Automated mode with a custom DNS SAN
./pki_gen.sh --automated --dns mis.mycompany.io

# Automated mode with custom DNS SAN and Minter CA CN
./pki_gen.sh --automated --dns mis.mycompany.io --minter-cn mycompany.io

# Interactive mode — prompts for all fields
./pki_gen.sh --interactive

# Let the script ask which mode to use
./pki_gen.sh

# Show help
./pki_gen.sh --help
```

### Certificate Chain

```
HTTPS CA (self-signed)
  └── HTTPS Server Certificate (signed by HTTPS CA)

Minter CA (self-signed)
  └── X.509 SVIDs (minted at runtime by MIS)
```

---

## 3. `svid-gen.sh` — SVID Certificate Generator

### Purpose

Mints X.509 SPIFFE Verifiable Identity Documents (SVIDs) for Margo principals by invoking `mis-cli` inside a running `margo-identity-service` Docker container. Supports two principal types:

| Principal    | SPIFFE ID Format                                                        |
|--------------|-------------------------------------------------------------------------|
| `wfm`        | `spiffe://<trust-domain>/margo/wfm/<wfm-id>`                           |
| `wfm-client` | `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`    |

Certificates are copied from the container to the host after minting.

### Prerequisites

- Bash 4+
- Docker installed and running
- Container named `margo-identity-service` must be running
- `mis-cli` binary available at `./mis-cli` inside the container

### Usage

```bash
./svid-gen.sh [OPTIONS]
```

| Flag                        | Description                                                        |
|-----------------------------|--------------------------------------------------------------------|
| `--interactive`             | Run in interactive mode (default if no flags given)                |
| `--automated`               | Run in automated mode                                              |
| `--principal <wfm\|wfm-client>` | **(Automated)** Principal type to generate SVID for            |
| `--spiffe-id <spiffeID>`    | **(Automated)** Full SPIFFE ID                                     |
| `--dns <name>`              | **(Automated)** DNS SAN to include; can be repeated for multiple   |
| `--help`, `-h`              | Show usage information                                             |

### Defaults

| Parameter      | Default                  |
|----------------|--------------------------|
| Trust Domain   | `margo.org`              |
| TTL            | `7776000` seconds (90 days) |
| DNS SANs       | None                     |

### Output

Certificates are copied from the container to the host in a directory named:

| Principal    | Host Output Directory                    |
|--------------|------------------------------------------|
| `wfm`        | `./x509svid-<wfm-id>` (interactive) / `./x509svid-wfm` (automated) |
| `wfm-client` | `./x509svid-<wfm-id>-<wfm-client-id>` (interactive) / `./x509svid-wfmclient` (automated) |

### Examples

```bash
# Interactive mode (default — no flags needed)
./svid-gen.sh
./svid-gen.sh --interactive

# Automated mode — mint SVID for a WFM
./svid-gen.sh --automated \
  --principal wfm \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm

# Automated mode — mint SVID for a WFM Client
./svid-gen.sh --automated \
  --principal wfm-client \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client

# Automated mode — WFM with DNS SANs
./svid-gen.sh --automated \
  --principal wfm \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm \
  --dns my-wfm.example.com \
  --dns api.my-wfm.example.com

# Show help
./svid-gen.sh --help
```

### Interactive Mode Walkthrough

When run interactively, the script guides you through the following steps:

1. **Trust Domain** — Enter your SPIFFE trust domain (e.g., `margo.org`)
2. **Principal Selection** — Choose `1` for WFM or `2` for WFM Client
3. **WFM ID** — Unique identifier for the Workflow Manager instance
4. **WFM Client ID** — *(WFM Client only)* Unique identifier for the client
5. **TTL** — Certificate lifetime in seconds
6. **DNS SANs** — Optional space-separated DNS names
7. **Confirmation** — Review summary and confirm before minting

---

## Typical End-to-End Workflow

```bash
# Step 1: Generate PKI (CA certificates and HTTPS server cert)
./pki_gen.sh --automated --dns mis.margo.org

# Step 2: Build MIS configuration
./confbuilder.sh --automated

# Step 3: Start the margo-identity-service container
# (using the generated certs and configuration.json)

# Step 4: Mint SVIDs for your principals
./svid-gen.sh --automated \
  --principal wfm \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm
```