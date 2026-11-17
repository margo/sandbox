
# Certificates Directory

> ⚠️ **WARNING: Development Use Only**
>
> The certificates in this directory are **pre-generated, self-signed certificates intended solely for local development and testing**.
> They **must not** be used in any staging or production environment.

---

## Why You Should Not Reuse These Certificates

- Private keys are committed to the repository and are **no longer secret**
- Anyone with repository access can impersonate your server or sign arbitrary SVIDs
- Self-signed certificates lack proper revocation mechanisms
- Subject fields, SANs, and validity periods are set to generic development defaults

**Always generate fresh certificates for any non-development deployment.**
See [`scripts/lib/mis/pki_gen.sh`](../scripts/lib/mis/pki_gen.sh) for tooling to generate your own PKI.

---

## Certificate Files

### SPIFFE SVID Generation (Minter CA)

Used by MIS to sign X.509 SVIDs issued via `mis mint x509`.

| File | Description |
|------|-------------|
| `ca.crt` | Minter CA certificate |
| `ca.key` | Minter CA private key |

### HTTPS Endpoint Serving

Used to secure the MIS REST API server with TLS.

| File | Description |
|------|-------------|
| `https-ca.crt` | HTTPS CA certificate |
| `https-ca.key` | HTTPS CA private key |
| `https-server.crt` | HTTPS server certificate (signed by `https-ca.crt`) |
| `https-server.key` | HTTPS server private key |
| `https-ca-trust-pin.txt` | SPKI pin of the `https-ca.crt` public key (SHA-256, Base64-encoded) |

---

## Directory Structure

This directory serves as a **reference structure** for mounting certificates into the MIS container via Docker Compose.

```
certs/
├── ca.crt                  # Minter CA certificate
├── ca.key                  # Minter CA private key
├── https-ca.crt            # HTTPS CA certificate
├── https-ca.key            # HTTPS CA private key
├── https-server.crt        # HTTPS server certificate
├── https-server.key        # HTTPS server private key
└── https-ca-trust-pin.txt  # SPKI pin of https-ca.crt public key
```

---

## HTTPS CA Trust Pin

The file `https-ca-trust-pin.txt` contains the **SPKI pin** (Subject Public Key Info) of the `https-ca.crt` public key in the format:

```
sha256/<base64-encoded-sha256-digest>
```

This pin can be used by clients to perform **certificate pinning** — verifying that the server's CA public key matches a known trusted value, independent of the certificate chain. This is useful for:

- Bootstrapping trust in environments where the CA certificate cannot be distributed in advance
- Additional validation layer on top of standard TLS verification

> ⚠️ This pin corresponds to the **development CA only** and must be regenerated alongside the certificates for any real deployment.

---

## Docker Compose Usage

The `certs/` directory is mounted into the container so MIS can read certificate files at runtime.
Paths inside the container must match what is declared in `configuration.json`.

```yaml
services:
  mis:
    image: mis:latest
    ports:
      - "18443:18443"
    volumes:
      - ./certs:/certs:ro
      - ./configuration.json:/configuration.json:ro
    command: start --config /configuration.json
```

The corresponding `configuration.json` should reference the mounted paths:

```json
{
  "ca": {
    "cert": "/certs/ca.crt",
    "key":  "/certs/ca.key"
  },
  "https": {
    "ca":        "/certs/https-ca.crt",
    "cert":      "/certs/https-server.crt",
    "key":       "/certs/https-server.key",
    "trust-pin": "/certs/https-ca-trust-pin.txt"
  }
}
```

---

## Generating Your Own Certificates

For any real deployment, generate fresh certificates using the provided script:

```bash
# Interactive — prompts for CN, SANs, organization, validity periods, etc.
./scripts/lib/mis/pki_gen.sh --interactive

# Automated — uses built-in defaults, no prompts
./scripts/lib/mis/pki_gen.sh --automated
```

Store the generated files in a **secure location outside the repository** and mount them into the container at runtime.

> 🔒 Never commit private keys (`*.key`) to source control.
