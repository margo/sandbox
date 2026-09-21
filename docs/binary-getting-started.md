# 🚀 Device Agent - Binary Getting Started Guide

This guide explains how to setup, configure, and run the `device-agent` binary.

The `device-agent` is responsible for managing and executing workloads on a device. It applies configurations and runs workloads based on the provided setup.

---

## 📌 Overview

This guide specifically covers the **device-agent binary**.

The binary requires:
- Configuration file (`config.yaml`)
- An X.509 SVID (certificate + private key) issued by a Margo Identity Service (MIS)
- The MIS HTTPS CA certificate

---

## ✅ Prerequisites

Ensure the following before proceeding:

- A running Margo Identity Service (MIS) — either your operator's own deployment or the sandbox MIS
- The required backend service (e.g., WFM) is running
- Observability stack is available as part of the Margo ecosystem (e.g., OTEL Collector, Grafana, Jaeger, Prometheus, etc.)

For MIS startup instructions, see [Build and MIS](./setup-guide.md#build-and-run-mis) for the sandbox deployment or [Run locally](../mis/README.md#run-locally) for the MIS binary.

👉 Note: Device-agent is not strictly dependent on the observability stack, but skipping it may result in non-compliance with Margo device requirements.

---

## 📌 Minimum Requirements

The device-agent is lightweight and can run on low-resource systems.

Approximate runtime footprint:

- CPU: ~1–2 vCPU
- Memory: ~200–500 MB
- Disk: ~500 MB

👉 Note: These values are recommended for running the device-agent in a stable environment.  
If other components (e.g., WFM) are running on the same machine, additional resources should be allocated accordingly.

---

## ⚙️ Getting Started

### ✅ Step 1: Obtain Device Identity (X.509 SVID)

The device-agent requires an X.509 SVID issued by a MIS. Choose the path that matches your setup.

---

#### 🔹 Option A: Using Your Operator's MIS

If your operator provides a MIS, mint an X.509 SVID and key for the device using that MIS, and obtain the MIS HTTPS CA certificate from your operator.

Once you have the files, place them as follows:

```
./config/identity/payload-cert.pem   ← SVID certificate
./config/identity/payload-key.pem    ← SVID private key
./config/mis/https-ca.crt            ← MIS HTTPS CA certificate
```

> ⚠️ Ensure the MIS endpoint is reachable from the machine where the device-agent will run.

---

#### 🔹 Option B: Using the Sandbox MIS (Docker)

If you are using the MIS deployed via the sandbox (running as a Docker container named `margo-identity-service`), use the provided `svid-gen.sh` helper script in interactive mode:

```bash
./scripts/lib/mis/svid-gen.sh --interactive
```

The script will guide you through:

1. **Trust Domain** — e.g. `margo.org`
2. **Principal** — select `2) WFM Client` for a device agent identity
3. **WFM ID** — identifies the Workflow Manager this device belongs to
4. **WFM Client ID** — uniquely identifies this device within the WFM
5. **TTL** — validity period in seconds (default: 90 days / `7776000`)
6. **DNS SANs** — optional; press Enter to skip

On completion, certificates are copied from the container to a local directory named `x509svid-<wfm-id>-<client-id>/`. Copy them into place:

```bash
mkdir -p ./config/identity ./config/mis

cp x509svid-<wfm-id>-<client-id>/payload-cert.pem ./config/identity/
cp x509svid-<wfm-id>-<client-id>/payload-key.pem  ./config/identity/
```

Also copy the MIS HTTPS CA certificate (generated during MIS PKI setup — see `[MIS PKI Setup](../mis/README.md#pki-setup)`):

```bash
cp <path-to-mis-pki>/certs/https-ca.crt ./config/mis/https-ca.crt
```

---

#### 🔹 Option C: Using the Sandbox MIS (Binary)

If MIS is running as a binary (not via sandbox Docker deployment), use the `mis mint x509` command directly:

```bash
mis mint x509 \
  --spiffeID spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<device-id> \
  --ttl 7776000 \
  --outputDir ./config/identity
```

This produces two files in `./config/identity/`:

| File | Description |
|------|-------------|
| `payload-cert.pem` | X.509 SVID certificate |
| `payload-key.pem` | Corresponding private key |

Also copy the MIS HTTPS CA certificate (generated during MIS PKI setup — see `[MIS PKI Setup](../mis/README.md#pki-setup)`):

```bash
cp <path-to-mis-pki>/certs/https-ca.crt ./config/mis/https-ca.crt
```

> See `[MIS mint x509 documentation](../mis/README.md#mint-x509)` for full flag details.

---

### ✅ Step 2: Configure `config.yaml`

The `device-agent` uses a `config.yaml` file to define its runtime behavior. The `miaf` section controls identity and MIS trust settings.

#### Configuration Modes

Choose the mode that matches your deployment:

---

##### Mode 1: MIS Endpoint — Dynamic Trust Bundle (Most Common)

The agent connects to MIS to fetch and refresh the SPIFFE trust bundle dynamically, and MIS supports Discovery.

```yaml
miaf:
  x509:
    certPath: "./config/identity/payload-cert.pem"
    keyPath: "./config/identity/payload-key.pem"
  mis:
    endpoint: "https://<mis-host>:<port>"
    caPath: "./config/mis/https-ca.crt"
    cacheInterval: 60
```

---

##### Mode 2: Static Trust Bundle (No MIS Endpoint)

Use a pre-fetched trust bundle from disk. `trustDomain` is required in this mode.

```yaml
miaf:
  x509:
    certPath: "./config/identity/payload-cert.pem"
    keyPath: "./config/identity/payload-key.pem"
  mis:
    trustDomain: "margo.org"
    trustBundle:
      path: "./config/mis/trust-bundle.json"
```

---

##### Mode 3: Static Trust Bundle with MIS Endpoint (Fallback)

Combines a local trust bundle with a live MIS endpoint for refreshes.

```yaml
miaf:
  x509:
    certPath: "./config/identity/payload-cert.pem"
    keyPath: "./config/identity/payload-key.pem"
  mis:
    endpoint: "https://<mis-host>:<port>"
    caPath: "./config/mis/https-ca.crt"
    cacheInterval: 60
    trustBundle:
      uri: "/.well-known/spiffe/bundle.json"
      path: "./config/mis/trust-bundle.json"
```

---

#### Configuration Rules at a Glance

| Rule | Details |
|------|---------|
| `miaf` block | Mandatory — must not be omitted |
| `x509.certPath` + `x509.keyPath` | Always required |
| `mis.endpoint` + `mis.caPath` | Must be provided together — one without the other is invalid |
| `trustBundle.path` | Makes `endpoint`, `caPath`, and `trustBundle.uri` optional |
| Without `trustBundle.path` | `endpoint` + `caPath` are required |
| `trustDomain` | Required when no `endpoint` + `caPath` (static mode) |
| `trustBundle.uri` | If provided, `endpoint` + `caPath` must also be present |

Also configure the runtime and WFM connection in `config.yaml`:

```yaml
wfm:
  sbiUrl: https://<wfm-host>:<port>/v1alpha2/margo

runtimes:
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
  # - type: DOCKER
  #   docker:
  #     url: unix:///var/run/docker.sock
```

👉 Review all paths and values before running the binary.

---

### ✅ Step 3: Run the Binary

```bash
./device-agent --config config/config.yaml
```

---

### ✅ Step 4: Verify

#### 🔹 Check Running Process

```bash
ps -ef | grep device-agent
```

Example output:
```
root     7978 ... ./device-agent --config config/config.yaml
```

👉 If the process is visible, the device-agent is running successfully.

---

## ✅ Execution Flow Summary

1. Obtain X.509 SVID from MIS (operator's MIS, sandbox Docker via `svid-gen.sh`, or sandbox binary via `mis mint x509`)
2. Place SVID and MIS HTTPS CA certificate in `config/`
3. Configure `config.yaml` (MIAF + runtime + WFM settings)
4. Run the binary
5. Verify using process or logs

---

## ⚠️ Important Notes

- The `miaf` block is mandatory — the agent will not start without it
- `mis.endpoint` and `mis.caPath` must always be configured together
- Ensure the MIS endpoint is reachable from the device-agent machine
- Missing or incorrect certificate paths will cause startup failure
- Use logs for debugging issues
` ` ` `