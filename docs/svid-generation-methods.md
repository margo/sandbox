# SVID Generation Methods

This document describes the two supported methods for generating X.509 SVIDs (SPIFFE Verifiable Identity Documents) in the Margo Code First Sandbox. Both methods produce a signed `payload-cert.pem`; they differ in who generates and holds the private key. This guide describes SVID generation methods when MIS is deployed using [Sandbox Setup Guide](./setup-guide.md). For SVID Generation when deploying as binary, refer to [MIS Documentation](../mis/README.md#mint-x509)

| Method | Who generates the key | MIS receives | MIS produces |
|---|---|---|---|
| **SPIFFE ID flow** | MIS (inside the container) | SPIFFE ID string | Certificate + private key |
| **CSR flow** | Operator (on the Principal) | Pre-generated CSR containing SPIFFE ID | Certificate only |

Choose the **SPIFFE ID flow** for simplicity. Choose the **CSR flow** when your security policy requires that the private key never leave the device or principal that will use it.

---

## Method 1 — SPIFFE ID Flow (MIS generates key and certificate)

MIS generates both the ECDSA P-256 private key and the signed X.509 SVID certificate inside the `margo-identity-service` container. The key and certificate are then copied to the host.

### Prerequisites

- `margo-identity-service` Docker container is running and preferably using [Sandbox Setup Guide](./setup-guide.md#build-and-run-mis).
- `mis-cli` binary is available inside the container at `./mis-cli`.

### Step 1 — Run the SVID generator

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash mis.sh
```

- A menu will appear.
- Type `6` and press Enter.
- Choose: `Option 6: Generate SVID`

### Step 2 — Follow the interactive prompts

When prompted for the **Generation Method**, choose:

```
1) SPIFFE ID
```

Then provide the remaining inputs:

| Prompt | Example value | Notes |
|---|---|---|
| Trust Domain | `margo.org` | Must match `trustDomain` in MIS configuration |
| Principal | `1` (WFM) or `2` (WFM Client) | |
| WFM ID | `my-wfm` | Unique identifier for the WFM instance |
| WFM Client ID | `my-client` | WFM Client only |
| TTL | `7776000` | Certificate lifetime in seconds (default: 90 days) |
| DNS SANs | *(optional)* | Space-separated DNS names |

Review the summary and confirm to mint the SVID.

### Step 3 — Locate the output files

Output is written to a subdirectory of the current working directory:

| Principal | Output directory |
|---|---|
| WFM | `./x509svid-<wfm-id>/` |
| WFM Client | `./x509svid-<wfm-id>-<wfm-client-id>/` |

Each directory contains:

```
payload-cert.pem   # X.509 SVID certificate  (0644)
payload-key.pem    # ECDSA P-256 private key  (0400)
```

### Automated mode (non-interactive)

```bash
# WFM
./scripts/lib/mis/svid-gen.sh --automated \
  --principal wfm \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm

# WFM Client
./scripts/lib/mis/svid-gen.sh --automated \
  --principal wfm-client \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client
```

---

## Method 2 — CSR Flow (Operator generates key; MIS signs certificate only)

The operator generates the private key and a Certificate Signing Request (CSR)
locally. Only the CSR is submitted to MIS. MIS validates the CSR, signs the
certificate, and returns `payload-cert.pem`. **The private key never leaves the
host.**

### Prerequisites

- `openssl` installed and available in `PATH`.
- `margo-identity-service` Docker container is running.
- `mis-cli` binary is available inside the container at `./mis-cli`.

### Step 1 — Generate the private key and CSR

The sandbox scripts (`wfm.sh` and `device-agent.sh`) include a built-in option to generate the private key and CSR directly on the host, without requiring manual `openssl` invocation.

#### Option A — Using the sandbox menu (recommended)

**For WFM** (`wfm.sh`):

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash wfm.sh
```

- Choose: `8) Generate Private Key & CSR`
- Enter the SPIFFE ID when prompted (format: `spiffe://<trust-domain>/margo/wfm/<wfm-id>`)

Output is written to `$HOME/wfm-identity/`:

```
payload-key.pem    # ECDSA P-256 private key — keep secure, never share with MIS
payload-req.csr    # CSR with SPIFFE ID embedded as URI SAN
```

**For WFM Client / Device Agent** (`device-agent.sh`):

```bash
cd $HOME/workspace/sandbox/scripts
bash device-agent.sh
```

- Select device type (`docker` or `k3s`) when prompted.
- Choose: `12) Generate Private Key & CSR`
- Enter the SPIFFE ID when prompted (format: `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`)

Output is written to `$HOME/certs/compose-identity/` (Docker) or `$HOME/certs/helm-identity/` (k3s):

```
payload-key.pem    # ECDSA P-256 private key — keep secure, never share with MIS
payload-req.csr    # CSR with SPIFFE ID embedded as URI SAN
```

#### Option B — Using the CLI (non-interactive)

```bash
# WFM
bash wfm.sh generate-csr

# WFM Client (Docker device)
bash device-agent.sh docker generate-csr

# WFM Client (k3s device)
bash device-agent.sh k3s generate-csr
```

#### Option C — Using the helper script directly

```bash
cd $HOME/workspace/sandbox/scripts
bash scripts/lib/mis/csr_gen.sh \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm
```

For a WFM Client:

```bash
bash scripts/lib/mis/csr_gen.sh \
  --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client
```

Optional subject field overrides:

| Flag | Default | Description |
|---|---|---|
| `--cn <name>` | `mis.margo.org` | Common Name |
| `--org <name>` | `Margo` | Organization |
| `--ou <name>` | `Margo Sandbox` | Organizational Unit |
| `--country <code>` | `IN` | 2-letter country code |
| `--state <name>` | `Haryana` | State or province |
| `--locality <name>` | `Gurugram` | Locality / city |
| `--key-out <file>` | `payload-key.pem` | Output path for the private key |
| `--csr-out <file>` | `payload-req.csr` | Output path for the CSR |

> ⚠️ The private key is written with `0600` permissions. Store it securely.
> It is **never sent to MIS**.

#### Bringing your own CSR

If you prefer to generate the CSR using your own tooling (e.g., `openssl` directly,
an HSM, or an enterprise PKI tool), ensure the CSR meets the following requirements:

| Requirement | Value |
|---|---|
| Key algorithm | ECDSA P-256 (`prime256v1`) recommended |
| URI SAN | Exactly one `URI:<SPIFFE_ID>` entry |
| SPIFFE ID format | `spiffe://<trust-domain>/margo/<path>` |
| Basic Constraints | `CA:FALSE` |
| Key Usage | `critical, digitalSignature` |
| Extended Key Usage | `clientAuth, serverAuth` |
| CSR signature | Must be valid |

### Step 2 — Submit the CSR to MIS

Run the SVID generator and select the CSR flow:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash mis.sh
```

- Type `6` and press Enter.
- Choose: `Option 6: Generate SVID`

When prompted for the **Generation Method**, choose:

```
2) CSR
```

Then provide:

| Prompt | Example value |
|---|---|
| Principal | `1` (WFM) or `2` (WFM Client) |
| CSR file path | `/path/to/payload-req.csr` |
| TTL | `7776000` |
| DNS SANs | *(optional)* |

Review the summary and confirm to mint the SVID.

### Step 3 — Locate the output file

Output is written to a subdirectory named after the CSR filename stem:

```
./x509svid-<csr-filename-stem>/
└── payload-cert.pem   # Signed X.509 SVID certificate (0644)
```

No `payload-key.pem` is written — you already hold the key from Step 1.

### Automated mode (non-interactive)

```bash
# WFM
./scripts/lib/mis/svid-gen.sh --automated \
  --principal wfm \
  --csr-path ./payload-req.csr

# WFM Client
./scripts/lib/mis/svid-gen.sh --automated \
  --principal wfm-client \
  --csr-path ./payload-req.csr
```

---

## Comparison Summary

| | SPIFFE ID flow | CSR flow |
|---|---|---|
| Private key generated by | MIS (inside container) | Operator (on host) |
| Private key leaves the host | Yes (copied out of container) | No |
| Files produced | `payload-cert.pem` + `payload-key.pem` | `payload-cert.pem` only |
| Suitable for | Sandbox / PoC / quick setup | Security-sensitive deployments |
| Key/CSR generation | N/A | `wfm.sh` option 8, `device-agent.sh` option 12, or `csr_gen.sh` directly |
| Helper script | `svid-gen.sh --spiffe-id` | `csr_gen.sh` + `svid-gen.sh --csr-path` |

---

## Related Documentation

- `[Identity Lifecycle and Operator Playbooks](./identity-lifecycle.md)`
- `[Setting Up the Code First Sandbox](./setup-guide.md)`
- `[MIS CLI Reference](../mis/README.md)`
- `[Scripts Reference — svid-gen.sh, csr_gen.sh](../scripts/lib/mis/README.md)`
