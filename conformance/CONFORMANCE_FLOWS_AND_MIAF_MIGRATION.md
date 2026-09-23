# Margo Conformance — End‑to‑End Flows, Terminology, and the MIAF Migration

**Last updated:** 2026‑09‑22
**Author:** conformance team (nitparihar)
**Spec baseline:** `1.0.0-rc.2`→`rc.3` (SBI spec bumped, see Part 5.8) / `pre-draft` branch of `github.com/margo/specification`
**Reference implementation cross‑checked:** `feature/miaf` branch of `github.com/margo/sandbox` (the project's own code‑first sandbox — see Parts 5.7 and 5.8)
**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) (current implementation detail), [SYMPHONY_WFM_CONFORMANCE_GAPS.md](SYMPHONY_WFM_CONFORMANCE_GAPS.md)

**2026‑09‑17 update:** Parts 5.7, 7, and 8 revised after reading the actual
`feature/miaf` reference code (not just the spec pages) and after a decision
from the manager: **we do not build or run our own MIS.** For WFM‑supplier
testing we just fetch the WFM's published trust bundle once at suite startup;
for device‑supplier testing we host only the 2 discovery endpoints (a static
stub, not a real MIS) once at suite startup. See Part 5.7 and the rewritten
Part 7.

**2026‑09‑22 update:** `feature/miaf` gained ~29 new commits since the
2026‑09‑17 review — a second pass (Part 5.8) confirms the 5 SBI endpoints and
MIS's 2 endpoints are unchanged, but the client‑side trust machinery matured
significantly: the authorized‑WFM allowlist is now genuinely hot‑reloadable
(and HTTP keep‑alives were deliberately disabled to force per‑request
re‑authorization), RFC 9457 problem‑details parsing is now real on the device
side, and status reports gained a new required `adoptedManifestVersion`
field. Two gaps/bugs worth tracking are flagged in Part 5.8 and a new Part 8
question. Part 7.1 gained a new capability (H) for the hot‑reload scenario.

---

## 0. How to read this document

This document has three jobs:

1. **Explain every conformance flow, per persona, with every nuance** — what the suite
   sends, what it checks, why, and the quirks we have hit against real
   implementations. This is Parts 3–4.
2. **Give you a plain‑language glossary** — one line + one example per term
   (digest, blob, bundle, manifest, byte, metadata, SVID, Trust Bundle, …). Part 2.
3. **Analyse the spec change** — the latest spec (`rc.2`) **replaces the onboarding
   flow completely** with the **Margo Identity and Authorization Framework (MIAF)**.
   The current suite is built for the *old* flow. Parts 5–7 explain the new flow in
   detail and lay out exactly what has to change in the conformance suite.

If you only read one thing: **Part 5 (the new onboarding flow) and Part 6 (the
old → new change matrix).**

---

## 1. The one‑paragraph summary of what changed

The **old** Margo Management Interface authenticated each device by (a) an
`POST /onboarding` call where the device uploads a self‑signed X.509 certificate
and gets back a `clientId`, then (b) **HTTP Message Signatures (RFC 9421)** — every
request carried `Signature`, `Signature-Input` and `Content-Digest` headers, and the
`clientId` sat in every URL path (`/api/v1/clients/{clientId}/…`). Transport was
plain server‑side TLS.

The **new** spec deletes all of that. Identity is now an **X.509‑SVID** (a SPIFFE
identity certificate) that an **operator provisions out of band** before the device
ever contacts the WFM — there is **no onboarding API call**. Authentication is
**mutual TLS (mTLS)**: the device and the WFM each present their SVID in the TLS
handshake and validate the other against a shared **Trust Bundle**. Because the
caller is identified by the SPIFFE ID inside its client certificate, the `clientId`
disappears from every URL (`/api/v1/capabilities/{deviceId}`,
`/api/v1/deployments`, …). No request signing, no `Content-Digest`, no `Signature`
headers. A new component, the **Margo Identity Service (MIS)**, publishes the Trust
Bundle and an optional discovery document.

**Impact on the suite:** the request‑signing engine (`run_wfm_scenarios.js`
RFC 9421 code, the Go `signRequest`), the onboarding scenarios, and every
`/clients/{clientId}/…` path are obsolete for `rc.2`. We need an mTLS client, a
small **MIS mock** (or fixture SVIDs + Trust Bundle), and rewritten scenario files.
See Part 7 for the migration plan.

---

## 2. Terminology — one line + one example each

### 2A. Content‑addressing / HTTP plumbing terms

| Term | One line | Example |
|---|---|---|
| **byte / bytes** | The raw 8‑bit units that make up a file or HTTP body — "the exact bytes" means the file as‑is, no re‑encoding, no whitespace changes, no newline normalisation. | A 512‑byte tar.gz is 512 bytes; if a proxy gzips then ungzips it and adds a trailing `\n`, the bytes changed even though the "content" looks the same. |
| **digest** | A fixed‑length fingerprint of some bytes, produced by a hash function (Margo uses SHA‑256). Same bytes → same digest; one bit different → completely different digest. Written `<algorithm>:<hex>`. | `sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08` |
| **content‑addressed** | You fetch a thing *by its digest*, not by a name. The URL contains the digest, so the response can never be the "wrong" content — if it doesn't hash to that digest, the server 404s. | `GET /api/v1/bundles/sha256:9f86…a08` — ask for the bundle whose bytes hash to that value. |
| **blob** | "Binary Large OBject" — an opaque lump of bytes the transport layer doesn't look inside (an image layer, a tar.gz, a YAML file treated as raw text). In OCI registries every file is stored as a blob addressed by its digest. | The gzipped tar of an application package, stored in the registry as one blob. |
| **manifest** | A small structured document (JSON/YAML) that *lists and points at* other things by digest — it is the "table of contents". It does not contain the payload, only references + metadata. | The desired‑state manifest: `{ "manifestVersion": 4, "bundle": {…digest…}, "deployments": [ {…digest…} ] }` — it names two blobs by digest; you fetch each separately. |
| **bundle** | In Margo: one **tar.gz blob** that contains *all* the `ApplicationDeployment` YAML files a device currently needs, so the device can pull its whole desired state in a single request instead of N requests. Media type `application/vnd.margo.bundle.v1+tar+gzip`. | `bundle.tar.gz` → unpacks to `deployment-app-a.yaml`, `deployment-app-b.yaml`. |
| **metadata** | Data *about* data — descriptive fields that aren't the payload itself: size, media type, creation time, labels, name, version. | In `bundle: { "mediaType": "...", "digest": "sha256:…", "sizeBytes": 20480, "url": "..." }` everything except the actual tar.gz bytes is metadata. |
| **media type / MIME type** | A string that says "what kind of bytes these are" so the receiver parses them correctly. Sent in `Content-Type` / `Accept`. Margo defines its own. | `application/vnd.margo.manifest.v1+json`, `application/vnd.margo.bundle.v1+tar+gzip`, `application/yaml`, `application/problem+json` |
| **ETag** | A short server‑issued token identifying *this exact version* of a resource. In Margo it is the resource's digest, quoted. The client sends it back in `If-None-Match` to ask "changed since this?". | Response: `ETag: "sha256:9f86…a08"`. Next request: `If-None-Match: "sha256:9f86…a08"` → `304 Not Modified` if unchanged. |
| **conditional GET / 304** | A GET that says "only send the body if it changed" (via `If-None-Match`). If unchanged the server returns `304 Not Modified` with **no body** — saves bandwidth on polling. | Device polls `/deployments` every 15 s; 99% of the time gets a bodiless `304`. |
| **`Cache-Control: immutable`** | HTTP header meaning "this will never change, cache it forever." Correct for content‑addressed blobs (a digest URL's bytes can't change). **Wrong** for the manifest (its `manifestVersion` increments over time). | Bundle response: `Cache-Control: public, max-age=31536000, immutable` ✅. Manifest response with `immutable` ❌ (MI‑035 forbids it). |
| **`manifestVersion`** | A monotonic 64‑bit counter on the desired‑state manifest. Every new manifest for a client has a strictly greater value. Stops a device from rolling back to a stale/replayed manifest. | Manifest goes `1 → 2 → 3`; a device that has `3` must reject a manifest claiming `2`. |
| **`adoptedManifestVersion`** | *(new in rc.2 status API)* The manifest version the device has actually **taken up and started applying** — reported back in every status update so the WFM knows how far the device has caught up. Independent of success/failure. | WFM publishes v5; device is still applying v4 → device reports `adoptedManifestVersion: 4`. |
| **`application/problem+json` (RFC 9457)** | The standard error body shape: `{ type, title, status, detail, instance }` plus Margo extensions `retryable`, `backoffStrategy`, `errors[]`. Replaces ad‑hoc `{"Error": "..."}`. | `{ "type": "https://docs.margo.org/specification/problem-types#semantic-error", "title": "Semantic Error", "status": 422, "errors": [ … ] }` |

### 2B. Identity / MIAF terms (the new stuff)

| Term | One line | Example |
|---|---|---|
| **MIAF** | *Margo Identity and Authorization Framework* — the new common scheme for **who a component is** (identity), **proving it** (authentication), and **what it may do** (authorization). Built on SPIFFE. | "Authentication is mTLS with an X.509‑SVID; authorization is a local policy check on the peer's SPIFFE ID." |
| **SPIFFE** | An existing open standard for machine identity (not people). Margo adopts it instead of inventing its own. | The SPIFFE project also gives you SPIRE, a ready‑made identity server. |
| **Trust Domain** | The security boundary inside which identities are issued and mutually trusted — usually one per site/operator. Everything in it validates against the *same* trust material. | `factory.example` — a factory's WFM and all its devices share this Trust Domain. |
| **SPIFFE ID** | The name of one identity, as a URI: `spiffe://<trust-domain>/<path>`. Margo paths must start `/margo/`. | WFM: `spiffe://factory.example/margo/wfm/line7`  ·  device client: `spiffe://factory.example/margo/wfm/line7/client/press-42` |
| **SVID** | *SPIFFE Verifiable Identity Document* — the credential that proves you hold a SPIFFE ID. Binds the SPIFFE ID to a key pair. | The device's SVID = its identity "passport". |
| **X.509‑SVID** | The SVID as an **X.509 certificate** with the SPIFFE ID in the certificate's **URI SAN** field. This is what's presented in the TLS handshake. | A normal‑looking `.crt` whose `Subject Alternative Name: URI:spiffe://factory.example/margo/wfm/line7/client/press-42`. |
| **URI SAN** | The "Subject Alternative Name" X.509 extension, of type URI — where the SPIFFE ID lives inside the cert. Exactly **one** URI SAN is allowed in an SVID. | `X509v3 Subject Alternative Name: URI:spiffe://factory.example/margo/wfm/line7` |
| **Trust anchor** | A root (or intermediate) CA certificate that you have decided to trust. An SVID is valid if its chain leads up to a trust anchor you hold. | The `factory.example` root CA cert. |
| **Trust Bundle** | The published *set of trust anchors* for a Trust Domain, as a **SPIFFE bundle = a JWK Set (RFC 7517)**. Each anchor is a JWK with `"use":"x509-svid"` and the cert in `x5c`. Carries `spiffe_sequence` (version) and `spiffe_refresh_hint` (how often to re‑fetch, seconds). | `{ "spiffe_sequence": 3, "spiffe_refresh_hint": 3600, "keys": [ { "use":"x509-svid", "x5c":["MIIB…"] } ] }` |
| **MIS** | *Margo Identity Service* — the authority **role** in a Trust Domain that issues SVIDs and serves the Trust Bundle + discovery document over HTTPS. Can be a self‑signed CA, an intermediate under enterprise PKI, or SPIRE. It is a role, not a mandated product. | `https://mis.factory.example` serving `/.well-known/margo` and `/.well-known/spiffe/bundle.json`. |
| **Discovery document** | *(optional)* A tiny JSON at `GET /.well-known/margo` telling a client the `trustDomain` and the `trustBundleUri`. If omitted, the operator configures the bundle URI directly. | `{ "trustDomain": "factory.example", "trustBundleUri": "https://mis.factory.example/.well-known/spiffe/bundle.json" }` |
| **Initial trust bootstrap** | How a client trusts the MIS *the very first time*, before it has any SVID: either pre‑configured PKI anchors (+ DNS name check, RFC 9525) or an operator‑provisioned **pin**, or the bundle is delivered fully out of band. **Never** trust‑on‑first‑use. | Pin = `sha256/base64(SubjectPublicKeyInfo)` of the MIS cert, shipped in the device image. |
| **mTLS (mutual TLS)** | Both ends of the TLS connection present a certificate and validate the other's. Old flow: server‑TLS only (only the device checked the WFM). New flow: mTLS (both check). TLS 1.3 default, 1.2 allowed as non‑default fallback. | Handshake: device sends its X.509‑SVID, WFM sends its X.509‑SVID, each validates the other against the Trust Bundle. |
| **Principal** | Any non‑human Margo component that holds (or is being given) a SPIFFE identity — a WFM, a WFM Client. | The device's WFM Client process is a principal. |
| **Verifier** | A principal *in the act of* checking a peer's SVID and then applying its **local** authorization policy. There is no central auth server. | The WFM, when a device connects, is the verifier. |
| **Accepted‑client policy** | The WFM operator's **allow‑list** of which clients may actually use the API. Being in the Trust Domain / namespace is necessary but **not sufficient** — the operator must also add you. Removing you here revokes access at the app layer without touching the cert. | "Accept `spiffe://factory.example/margo/wfm/line7/client/*`" or a list of exact client IDs. |
| **`wfm-id` / `wfm-client-id`** | The path segments in the SPIFFE IDs. `wfm-id` names the WFM (operator‑assigned, avoids multi‑vendor collisions); `wfm-client-id` names one client's relationship to that WFM. Both: non‑empty, only `A–Z a–z 0–9 . - _`, not `.`/`..`, unique, stable. | `line7` and `press-42`. |
| **Enrollment / Renewal / Revocation / Re‑issuance** | The four operator‑driven lifecycle phases for an SVID (the fifth, **Active**, is the normative protocol surface). Enrollment = mint first SVID (preferably from a CSR so the private key never leaves the device); Revocation = allow‑list removal *or* Trust Bundle anchor rotation (no CRL/OCSP). | Renewal: operator mints a replacement SVID with the *same* SPIFFE ID before the old one expires. |
| **CSR** | *Certificate Signing Request* — the device generates its own key pair and sends a request; the MIS signs it into an SVID. Keeps the private key on the device (TPM/secure element). Any subject/SAN in the CSR is advisory — the MIS overrides it. | `openssl req -new -key device.key -out device.csr` (with the SPIFFE ID as a URI SAN, which the MIS then authoritatively sets). |
| **late binding** | A device isn't tied to one WFM vendor at manufacture time — because identity is Trust‑Domain‑level, the same device can be pointed at any Margo‑compatible WFM in that domain later. | Ship a generic device; the site operator decides at install time which WFM it talks to. |
| **see‑thru gateway** | A device that fronts child devices but hosts no workloads itself — it reports its own capabilities and **omits** the hosting fields, and relays child capabilities/status. Child `deviceId`s are hierarchical (`gateway/child`). | `press-line/press-42` — `press-42` sits behind the `press-line` gateway. |

### 2C. Margo workload terms (mostly unchanged, but you asked)

| Term | One line | Example |
|---|---|---|
| **Application** | One or more **Components** described by an **Application Description**, shipped in an **Application Package**. | "Line Monitor" app = a UI component + a data‑collector component. |
| **Component** | One deployable piece of software — a **Helm chart** or a **Compose archive**. | The data‑collector component, packaged as an OCI artifact. |
| **Workload** | A running instance of a Component on a device. | The data‑collector container actually running on `press-42`. |
| **Application Package** | A folder in a Git repo: the Application Description + references to Components + resources (icon, docs). | `git.example/apps/line-monitor/` |
| **Application Description** | The vendor's YAML declaring the app, its Components, and its configurable **parameters** (with schema, defaults, `immutable`). `kind: ApplicationDescription`. | Declares a `pollIntervalSeconds` parameter, integer, default 30. |
| **ApplicationDeployment** (a.k.a. desired‑state manifest / "the YAML") | The WFM‑produced YAML telling **one device** to run **one app** with concrete parameter values and a **deployment profile**. `kind` per LinkML `DesiredState`. Fetched at `GET /deployments/{deploymentId}/{digest}`. | `spec.applicationId: line-monitor`, `spec.deploymentProfile.type: compose`, `spec.parameters: [...]`. |
| **deployment profile** | The `compose` or `helm` half of an ApplicationDeployment — lists the Components with their OCI `repository` + `revision` and behaviour flags. | `type: compose`, `components: [ { name, properties: { repository: oci://…, revision: 1.2.0, wait: true } } ]` |
| **`wait`** | Per‑component boolean (`spec.deploymentProfile.components[].properties.wait`, default `true`). `true` = the client blocks until the install reports healthy before moving on; `false` = fire‑and‑forget. Also `timeout: "##m##s"`. | `wait: false` on a slow batch job so the rest of the deployment proceeds. |
| **desired state / state‑seeking / reconciliation** | The WFM only ever declares *what should be true* (the manifest). The device continuously compares that to *what is true* and converges — installing, updating, removing as needed. No imperative "install X now" command. | Manifest drops app‑b → device sees app‑b gone from desired state → device uninstalls app‑b → reports `state: removed`. |

---

## 3. The two personas (unchanged concept)

| | **WFM‑supplier** | **device‑supplier** |
|---|---|---|
| Question it answers | "Is my **WFM** spec‑conformant?" | "Is my **device agent / WFM Client** spec‑conformant?" |
| Under test (real) | The WFM (e.g. Symphony) | The device agent binary |
| Played by the suite (mock) | A conformant device client | A conformant WFM |
| Runner | `wfm-supplier/run_wfm_scenarios.js` (Node) — drives HTTP at the real WFM | `device-supplier/…` Go: `cmd/device-supplier/main.go` (mock WFM) + `run_tests.go` (a simulated device, for our own regression) |
| Scenario format | Postman collection **and** custom declarative JSON | custom declarative JSON |
| Consolidated file | `testcases/wfm-core/postman_collection.json` | `testcases/device-conformance/test-scenarios.json` |
| Verdict logic | in the runner (Go / JS), never in scenario JSON | same |

Both personas exercise the **same protocol** from opposite sides, so a spec change
hits both runners.

---

## 4. CURRENT conformance flows (what the suite implements today — the OLD spec)

> This is the `pre-draft` state the suite was built against — RFC 9421 signing,
> `POST /onboarding`, `clientId` in the path. Kept here as the "before" picture.
> Full detail in [ARCHITECTURE.md](ARCHITECTURE.md) §6–§7.

### 4.1 WFM‑supplier — current end‑to‑end

```
suite (mock device)                                   real WFM under test
───────────────────                                   ───────────────────
1. GET  /api/v1/onboarding/certificate            →   200 { certificate: <b64 PEM> }   (no signature)
2. POST /api/v1/onboarding                        →   201 { clientId }                 (cert in body; sig optional)
     { apiVersion, kind:OnboardingRequest, certificate:<b64 PEM> }
   ── extract clientId; every later URL embeds it ──
3. POST /api/v1/clients/{clientId}/capabilities/{deviceId}   →  201                     (RFC 9421 signed + Content-Digest)
     { apiVersion:device.margo.org/v1alpha1, kind:DeviceCapabilitiesManifest, properties:{ id:{clientId}, … } }
4. PUT  …/capabilities/{deviceId}                 →   201                               (signed)
5. GET  /api/v1/clients/{clientId}/deployments    →   200 { manifestVersion, bundle, deployments }   (signed)
     Accept: application/vnd.margo.manifest.v1+json      ETag: "sha256:…"
6. GET  …/deployments  If-None-Match: "sha256:…"  →   304
7. GET  /api/v1/clients/{clientId}/bundles/{digest}          →  200 tar.gz  |  404      (signed)
8. GET  …/deployments/{deploymentId}/{digest}     →   200 application/yaml  |  404      (signed)
9. POST …/deployments/{deploymentId}/status       →   200 { acknowledgement }           (signed)
     { …, deploymentId:{deploymentId}, status:{ state }, components:[…] }
```

**Auth model:** server‑side TLS + **HTTP Message Signatures (RFC 9421)** on every
call except step 1. Signed components: `@method @target-uri @authority` (+ `content-digest`
for bodies). `keyid = hex(SHA-256(PKIX-DER public key))`. ECDSA sig must be **IEEE
P1363 (64 bytes for P‑256)**, not DER. `Content-Digest: sha-256=:<b64>:`.

**Negative‑path scenarios the suite runs (WFM):**
missing/tampered signature → 401 (GET) / 400 (POST‑PUT); missing `Content-Digest` →
400; untrusted / fresh cert → 401/403; invalid enum in capabilities → 422; semantic
body error → 422; wrong `Accept` → 406; no `Accept` → default manifest media type;
unknown client → 404; conditional GET → 304; ETag digest grammar `^"sha256:[0-9a-f]{64}"$`
(MI‑034); manifest not `immutable` (MI‑035); ETag == sha256(body) (MI‑015);
`bundle: null` when zero deployments (MI‑009); `bundle.mediaType` (MI‑031).

**Known real‑WFM (Symphony) gaps** — see
[SYMPHONY_WFM_CONFORMANCE_GAPS.md](SYMPHONY_WFM_CONFORMANCE_GAPS.md): no body
validation (bad input → 201), wrong status codes (400 instead of 401/403/422),
content negotiation → 500 instead of 406, no `304`, manifest marked `immutable`.

**Blocked scenarios** (need a pre‑provisioned deployment the suite can't create):
MI‑005, MI‑014, MI‑024, MI‑025, MI‑031(non‑null), MI‑033, MI‑038 — parked in the
separate `wfm-multi-component` group (poll + operator assignment).

### 4.2 device‑supplier — current end‑to‑end

The mock WFM (`cmd/device-supplier/main.go`) implements the same endpoints and
expects the **real device agent** to:

```
real device agent                                     suite (mock WFM)
─────────────────                                      ────────────────
1. GET  /api/v1/onboarding/certificate            →   200 { certificate }
2. POST /api/v1/onboarding  { certificate }        →   201 { clientId }    (mock stores the cert's public key)
3. POST /api/v1/clients/{clientId}/capabilities    →   201                  (mock verifies RFC 9421 signature + Content-Digest)
4. GET  /api/v1/clients/{clientId}/deployments     →   200 manifest         (mock serves a fixture manifest; ETag)
5. GET  …/bundles/{digest}  or  …/deployments/{id}/{digest}  →  200         (mock serves fixture blobs by digest)
6. POST …/deployments/{id}/status                  →   200                  (mock checks the status manifest shape)
```

**Named negative fixtures** the mock injects (`cmd/device-supplier/negative_fixtures.go`)
— the device MUST reject each:
`FixtureUnsupportedDigestAlgorithm` (MI‑008), `FixtureBundleDigestMismatch`
(MI‑006/009), `FixtureDeploymentDigestMismatch` (MI‑019), `FixtureNonIncreasingManifestVersion`
(MI‑010), `FixtureWrongSizeBytes` (MI‑025/026 — digest still valid, so must NOT
reject). Plus `cert_validation.go` (`certRFC5280Violation`, MI‑004) and TLS‑trust
(MI‑018), and signature‑algorithm acceptance (MI‑012/013/014).

**Flexible‑order group:** the mock exposes test‑control endpoints (reset desired
state, assign/unassign a deployment) so the reconciliation lifecycle and multi‑app
scenarios can run without a human — these live in
`testcases/device-conformance/test-scenarios.json` (scenarios `flex-*`).

### 4.3 Current CR‑ID coverage (as of 2026‑09‑10)

- **WFM:** `testcases/wfm-core/postman_collection.json` — 11 items, `crIds` on each,
  ~15 MI‑ requirements covered/partially; report shows a "requirements exercised"
  count. Separate declarative groups: `wfm-multi-component` (AD‑001 + blocked MIs),
  `wfm-header-integrity` (MI‑026 ordering).
- **device:** `testcases/device-conformance/test-scenarios.json` — 27 scenarios,
  `crIds` at scenario + step level, ~30 MARGO‑DEV‑* requirements.

---

## 5. NEW conformance flow (rc.2 + MIAF) — in detail, in plain language

### 5.1 The mental model

Think of it like getting a **staff badge for a building**:

- **Old way:** you walk up to the building's front desk (`/onboarding`), hand them
  a photo of yourself (your self‑signed cert), and they print you a visitor
  sticker with a number on it (`clientId`). From then on you write that number on
  every form and sign every form by hand (RFC 9421).
- **New way:** *before* you ever go to the building, your employer's security
  office (**the MIS / operator**) issues you a real chip‑and‑PIN **staff badge**
  (**X.509‑SVID**) that already says who you are (**SPIFFE ID**). At the building
  door, you tap your badge and the door *also* shows you its badge (**mTLS**);
  both badges are checked against the same **list of valid issuers** (**Trust
  Bundle**). Inside, each room has its own **guest list** (**accepted‑client
  policy**) — a valid badge gets you in the door but not necessarily into every
  room. You never fill in your number or hand‑sign anything again.

### 5.2 The actors

| Actor | Role |
|---|---|
| **Operator** | The human/automation that runs the site. Provisions identities out of band, maintains the WFM's accepted‑client policy. |
| **MIS** (Margo Identity Service) | Issues SVIDs; serves the Trust Bundle (`/.well-known/spiffe/bundle.json`) and optional discovery doc (`/.well-known/margo`) over HTTPS. |
| **WFM Client** | The agent on the device. Holds a client X.509‑SVID. Connects to the WFM. |
| **WFM** | The fleet manager under management. Holds a WFM X.509‑SVID. Serves the Management Interface. |

### 5.3 The flow, end‑to‑end (from the spec's own step list)

**Phase 1 — Provisioning (out of band, before any connection)**

1. **Operator → WFM Client:** installs the client's **X.509‑SVID**, the **initial
   trust material** (PKI anchors *or* a cert pin for the MIS), and the **WFM
   endpoint URL** (routing only — no identity meaning).
   - SPIFFE ID minted: `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`
   - Preferred: the device generates its key + a **CSR**; the MIS signs it, so the
     private key never leaves the device (TPM/secure element).
2. **Operator → WFM:** installs the **WFM's X.509‑SVID**
   (`spiffe://<trust-domain>/margo/wfm/<wfm-id>`) and **adds the client to the
   WFM's accepted‑client policy** (by exact `wfm-client-id`, full SPIFFE ID, or a
   namespace wildcard).

**Phase 2 — Trust material retrieval (WFM Client ↔ MIS)** *(skippable if the Trust
Bundle was delivered fully out of band)*

3. **WFM Client → MIS:** `GET /.well-known/margo` (discovery document) — *optional*.
4. **MIS → WFM Client:** `{ trustDomain, trustBundleUri }`. Client checks
   `trustDomain` matches its own SPIFFE ID's domain (mismatch = config error).
5. **WFM Client → MIS:** `GET <trustBundleUri>` (the SPIFFE bundle). The TLS
   connection to the MIS is authenticated by the **initial trust bootstrap** (PKI
   anchors + DNS name per RFC 9525, or the pin) — *not* by an SVID, because the
   client can't validate MIAF SVIDs yet.
6. **MIS → WFM Client:** the SPIFFE bundle (JWK Set): trust anchors in `x5c`,
   `spiffe_sequence`, `spiffe_refresh_hint`.
   - Client rules: **replace** (never merge) its anchors with the retrieved set; a
     bundle with **zero** X.509 anchors MUST be rejected (fail closed); a failed
     retrieval leaves the cached bundle in force; refresh on the
     `spiffe_refresh_hint` interval (HTTP `304`/`Cache-Control` is only an
     optimisation *within* that interval, never a reason to skip a required
     refresh).

**Phase 3 — Mutual authentication (mTLS, WFM Client ↔ WFM)**

7. **WFM Client → WFM:** opens a TLS 1.3 connection, presenting its **client
   X.509‑SVID** (+ any intermediates inline).
8. **WFM → WFM Client:** presents its **WFM X.509‑SVID**.
9. **WFM Client validates the WFM:** chain to an anchor in the current Trust
   Bundle; not expired; leaf has `cA=false`, no `keyCertSign`/`cRLSign`, exactly
   one URI SAN; extract SPIFFE ID; it MUST be **exactly**
   `spiffe://<trust-domain>/margo/wfm/<wfm-id>` using the client's *own*
   trust‑domain and wfm‑id. Any mismatch → abort the connection.
10. **WFM validates the WFM Client:** same chain/leaf checks; SPIFFE ID MUST be
    exactly `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`
    with matching trust‑domain + wfm‑id; **then** the local **accepted‑client
    policy** check (namespace membership alone is *not* enough). Not accepted →
    the WFM denies (a `403` at the app layer, or refuses the connection).

**Phase 4 — Management Interface (normal operation, over the mTLS connection)**

11. **WFM Client → WFM:** `PUT /api/v1/capabilities/{deviceId}` — report device
    capabilities (first post‑connection call).
12. **WFM Client → WFM:** poll `GET /api/v1/deployments`, fetch bundles/YAML by
    digest, `POST /api/v1/deployments/{deploymentId}/status` — the state‑seeking
    loop, forever.

> **Note there is no step where the device uploads a cert and receives an
> identifier.** Identity is entirely a Phase‑1 out‑of‑band operator action.

### 5.4 The NEW Management Interface API surface

Base: `https://<wfm-host>/api/v1`. **Security: mTLS (TLS 1.3) with X.509‑SVID per
MIAF — on every endpoint.** Port **443**. **HTTP/1.1.**

| Method | Path | Purpose | Success | Errors |
|---|---|---|---|---|
| `PUT` | `/capabilities/{deviceId}` | Report/update capabilities (one method — no more POST‑then‑PUT) | `200` updated / `201` created | `400` malformed · `403` not‑authorized · `404` gateway‑not‑found (child before parent) · `422` semantic |
| `DELETE` | `/capabilities/{deviceId}` | Unregister a device | `204` | `403` · `404` device‑not‑found |
| `GET` | `/deployments` | Desired‑state manifest for the calling client | `200` `application/vnd.margo.manifest.v1+json` (+ `ETag`) · `304` | `406` unsupported `Accept` |
| `GET` | `/deployments/{deploymentId}/{digest}` | One `ApplicationDeployment` YAML, content‑addressed | `200` `application/yaml` (+ `ETag`, `Cache-Control: …immutable`) · `304` | `404` deployment‑not‑found |
| `GET` | `/bundles/{digest}` | The tar.gz bundle of all current YAMLs, content‑addressed | `200` `application/vnd.margo.bundle.v1+tar+gzip` (+ `ETag`, `…immutable`) · `304` | `404` invalid‑bundle |
| `POST` | `/deployments/{deploymentId}/status` | Report deployment/component status | `200` / `201` | `400` · `403` · `422` |

**`{deviceId}`** is now a **hierarchical path**: `id[/id[/id…]]` (RFC 3986
unreserved chars only) — a see‑thru gateway reports `gateway` then
`gateway/child`. A child manifest before its parent → `404` gateway‑not‑found.

**Manifest body (`UnsignedAppStateManifest`):**
```jsonc
{
  "manifestVersion": 4,                      // monotonic 64-bit
  "bundle": {                                // null when zero deployments
    "mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
    "digest":    "sha256:…",
    "sizeBytes": 20480,
    "url":       "https://…/api/v1/bundles/sha256:…"
  },
  "deployments": [                           // one entry per assigned app
    { "deploymentId": "…", "digest": "sha256:…", "sizeBytes": 812, "url": "…" }
  ]
}
```

**Status body (`DeploymentStatusManifest`) — changed:**
```jsonc
{
  "deploymentId": "…",
  "adoptedManifestVersion": 4,               // NEW — how far the device has caught up
  "deviceId": "gateway/child",               // only when reporting for a child
  "status":     { "state": "installed", "error": { … } },
  "components": [ { "name": "…", "state": "installed", "error": { … } } ]
}
```
`state` enum is now **`pending · installing · installed · removing · removed ·
failed`** (added `removing`/`removed`). Overall state = most severe component
state. Gateway error codes `101/102/103` reserved.

**Errors:** RFC 9457 `application/problem+json`, `type` from the
[problem‑types registry](https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/problem-types.md),
plus `retryable`, `backoffStrategy` (`none|fixed|exponential`),
`retryAfterSeconds`, `errors[]`. New problem type **`wfm-client-relationship-retired`**
→ `403` (operator retired the relationship). Clients MUST honour `Retry-After`
and use the `retryable` field, not the status code, to decide whether to retry.

**Caching / compression:** `ETag` + `If-None-Match` → `304`; `Cache-Control:
public, max-age=31536000, immutable` on digest‑addressed responses only (never the
manifest); `Accept-Encoding` / `Vary: Accept-Encoding` now in scope (gzip/br MAY).

**What is *gone*:** `POST /onboarding`, `GET /onboarding/certificate`,
`/clients/{clientId}/…` prefix, `Signature`, `Signature-Input`, `Content-Digest`
headers, `keyid`, the `401` "signature verification failed" path, `apiVersion` /
`kind` envelope on capabilities/status bodies (LinkML schemas, not k8s‑style
envelopes).

### 5.5 The NEW Trust Bundle API (the MIS side — a new conformance target)

Base: `https://<mis-host>`. **Security: none at the MIAF layer** (bootstrap trust
only).

| Method | Path | Purpose | Success | Errors |
|---|---|---|---|---|
| `GET` | `/.well-known/margo` | Discovery document | `200` `{ trustDomain, trustBundleUri }` (+ `ETag`) · `304` | `404` discovery‑document‑not‑found |
| `GET` | `/.well-known/spiffe/bundle.json` | SPIFFE bundle (JWK Set) | `200` (+ `ETag`) · `304` | `404` spiffe‑bundle‑not‑found (+ `retryAfterSeconds`) |

Bundle shape: `{ spiffe_sequence, spiffe_refresh_hint, keys: [ { kty, crv, x, y,
use:"x509-svid", x5c:[…] } ] }`. `trustBundleUri` MUST be `https://`.

### 5.6 Crypto rules (SVID profile)

- **Mandatory to implement:** ECDSA **P‑256 + SHA‑256**.
- **Also allowed:** Ed25519; RSA **≥ 3072‑bit** + SHA‑256 (RSASSA‑PSS recommended,
  PKCS#1 v1.5 only for certs/CSRs); ECDSA **P‑384 + SHA‑384**.
- **TLS:** 1.3 default; 1.2 only as non‑default fallback (RFC 9325); SSLv2/3,
  TLS 1.0/1.1 forbidden.
- **No CRL/OCSP.** Revocation = allow‑list removal or Trust Bundle anchor rotation
  + short SVID lifetimes.
- **Validity window** MUST be enforced (reject expired); small bounded clock skew
  MAY be tolerated.
- **Long‑lived connections:** a verifier SHOULD cap connection age relative to SVID
  lifetime and re‑evaluate authorization policy per request (so an allow‑list
  removal takes effect without waiting for the connection to drop).
- **Traffic‑inspecting proxies (NGFW/SWG/SASE) in‑path are unsupported** — the
  operator MUST exempt Margo mTLS endpoints. A TLS‑terminating proxy at the trust
  boundary that validates the SVID and forwards identity via RFC 9440
  `Client-Cert` IS allowed (proxy must strip inbound `Client-Cert*`; backend must
  only trust it from the proxy boundary).

### 5.7 What the reference implementation actually does (`feature/miaf` branch, confirmed from code)

Everything in Parts 5.1–5.6 above comes from the spec pages. This section
cross‑checks it against the Margo project's own reference code
(`github.com/margo/sandbox`, `feature/miaf` branch) — i.e. does the theory
match what actually got built. Short answer: **yes, closely** — the 6
Management Interface endpoints and the mTLS/SPIFFE model are exactly as
documented above. What the reference code adds is the *operational* detail
the spec pages don't spell out: how identity actually gets onto a device,
and what a device/WFM does at startup. In plain language:

**The Margo Identity Service (MIS) is much smaller than it sounds.** In the
reference implementation it is one small program with exactly **two** public,
unauthenticated HTTPS endpoints — `GET /.well-known/margo` (returns
`{ trustDomain, trustBundleUri }`) and `GET /.well-known/spiffe/bundle.json`
(returns the trust bundle). That's the entire "MIS" a conformance suite would
ever talk to over the network.

**Minting a certificate is a manual, offline step — not an API call.**
The reference MIS *can* mint an X.509‑SVID, but only via a command‑line tool
talking to a Unix socket on the machine MIS runs on (`mis mint x509
--spiffeID spiffe://…`). There is no network endpoint for it. In practice:
an operator runs that command once, gets back a cert + key file, and copies
them onto the device or WFM. **This confirms Phase 1 (Provisioning) in §5.3
is genuinely out‑of‑band** — it is not something our suite can or should
automate over HTTP. It's a one‑time manual setup step, exactly like
generating our own test fixtures today.

**A device no longer answers any requests — it only makes them.** In the old
flow the device exposed an onboarding endpoint the WFM (or our suite, playing
the WFM) could call. In the new reference implementation the device has **no
inbound HTTP server at all**. It only ever calls out: it fetches the trust
bundle from MIS, then opens an mTLS connection to the WFM and does the
capabilities/deployments/status polling loop as a client. **Practical impact
for us:** device‑supplier testing can no longer "call the device to check
something" — it has to stand up a fake WFM, let the real device connect to
it, and observe what the device does. That's the same mock‑WFM pattern our
`device-supplier` runner already uses today; it just now needs to speak mTLS
instead of RFC 9421.

**The 6 endpoints match §5.4 almost exactly.** Same 5 paths under
`/api/v1`, same manifest/status body shapes, same ETag/digest/immutable
rules — the only thing that changed is *how the caller proves who it is*
(mTLS + SPIFFE ID instead of a signed request). This is good news: our
existing declarative scenario JSON (payloads, expected statuses, digest/ETag
checks) mostly carries over unchanged; only the transport layer underneath
`request()` in `run_wfm_scenarios.js` needs to change.

**One real content change: the Application Package format moved on.**
`margo.yaml`'s `apiVersion` changed from `margo.org/v1-alpha1` to plain `v1`,
and the `kind: ApplicationDescription` field was dropped entirely — it's no
longer part of the schema. Our Application Registry fixture package
(`wfm-supplier/fixtures/margo-ctt-package/margo.yaml`, pushed to Harbor as
`margo-ctt-hello-world:1.0.0`) still uses the old `apiVersion` — needs a
one‑line fix + re‑push before we lean on it for MIAF‑era testing.

**The manager's decision on MIS, and why it doesn't block us.** We do not
need to build or operate a real MIS for conformance testing:
- **WFM‑supplier persona** (we act as the device talking to a real WFM): the
  WFM supplier tells us their trust domain / MIS URL, and **once at the start
  of a test run** we fetch `GET /.well-known/margo` then `GET
  <trustBundleUri>` from it and cache the result for every scenario in that
  run. We do not re‑fetch per test case.
- **Device‑supplier persona** (we act as the WFM a real device talks to): our
  own harness needs to serve only those same **2 endpoints**, once, so the
  device‑under‑test can bootstrap trust in us at its startup — this is a tiny
  static stub, not a real MIS, and it also only needs to run once per suite
  run, not per test case.
- Either way we still need **one** valid X.509‑SVID of our own to present in
  the mTLS handshake (minted manually, out of band, same one‑time‑setup
  posture as today's cert fixtures) — that doesn't change with this
  decision, it's a separate, smaller thing.

### 5.8 What's new since the last review (`feature/miaf`, as of 2026‑09‑22)

Re‑checked the reference implementation after ~29 new commits landed since
Part 5.7 was written. Everything in 5.7 still holds — same 2 MIS endpoints,
same 5 SBI endpoints, minting still manual/offline. What changed is that the
**client‑side trust machinery got a lot more real**, plus one new field.
In plain language:

**1. The "operator revokes a WFM's access" story is now a genuinely testable
behavior, not just a spec promise.** Two changes landed together to make
this true:
- The device now **watches its `authorized.json` allowlist file on disk**
  and reloads it live — no restart needed. (A bad entry rejects the whole
  reload and keeps the last‑good list, fail‑safe.)
- **HTTP keep‑alive was deliberately turned off** on the device's outbound
  connections to the WFM, with an explicit code comment saying why: *"so
  that Authorized client verification happens every time."* Every request
  now opens a brand‑new TLS connection, so the allowlist check re‑runs on
  every single call, not just once per long‑lived connection.

  **What this means for us:** we can now write a real conformance scenario
  — "remove a WFM's SPIFFE ID from the allowlist mid‑run, confirm the very
  next request from that WFM is rejected" — and expect it to actually pass
  against a conformant device, where a few weeks ago it would have required
  a restart to take effect.

**2. Error responses are now properly parsed as RFC 9457
`application/problem+json`, not just logged as flat strings.** The JSON
shape itself (`type`, `title`, `status`, `retryable`, `backoffStrategy`,
`errors[]`) was always defined in the spec — what's new is the device agent
actually reads those fields now and reacts differently based on them (e.g. a
specific "your relationship was retired" code path on a `403`). This
confirms the `problem_type_is` / `is_retryable` checks we planned to add
(Part 7.1‑F) are exactly the right shape to build.

**3. Status reports must now include `adoptedManifestVersion`.** Not a new
endpoint — a new *required field* on the existing status‑report call,
confirming which manifest version the device actually applied (so the WFM
can tell a device that's still catching up from one that's failing). Every
status‑report test case needs to check this field now, including on removal.

**Two things worth flagging upward before we build against this:**

- **Trust bundle rotation still doesn't really work.** A `refreshHint`
  setting was added and is now sent to clients, but the bundle's version
  counter (`spiffe_sequence`) is still hardcoded to `1` in the code, with a
  literal `// TODO` — there's nothing real to test for rotation yet. Not a
  blocker for us; just don't build a rotation test against this branch
  expecting it to mean anything yet.
- **Possible real unit bug in `refreshHint`.** It's documented/configured in
  *seconds* (default `500`), but the code appears to pass that number
  straight into a duration constructor with no conversion — which would make
  it nanoseconds, not seconds, unless a downstream library quietly corrects
  it. Worth a sanity check (or a bug report to the sandbox project) before
  anyone builds a test that depends on the actual refresh interval value.

---

## 6. OLD → NEW change matrix

| Concern | OLD (suite implements this) | NEW (`rc.2` + MIAF) | Suite impact |
|---|---|---|---|
| **Get an identity** | `POST /onboarding` with self‑signed cert → `clientId` | Operator provisions an X.509‑SVID out of band (CSR preferred). No API call. | Delete onboarding scenarios; add SVID fixtures / a minting helper. |
| **Get the WFM's CA** | `GET /onboarding/certificate` | `GET` MIS discovery + `GET` Trust Bundle (JWK Set), bootstrapped by pin/anchor | New MIS mock + discovery/bundle scenarios; Trust‑Bundle refresh/replace/fail‑closed tests. |
| **Per‑request auth** | RFC 9421: `Signature`, `Signature-Input`, `Content-Digest`; `keyid`; IEEE‑P1363 ECDSA | **mTLS** — client cert in the TLS handshake; nothing per‑request | Rip out `signRequest` / RFC 9421 engine; build an mTLS HTTP client that presents the SVID. |
| **Transport** | server‑side TLS only | **mutual** TLS 1.3, both validate against Trust Bundle | Runner must present a client cert AND validate the server SVID's SPIFFE ID shape. |
| **Who am I in a request** | `clientId` in every URL path + `properties.id` must equal it | SPIFFE ID in the client cert; **no `clientId` anywhere** | Every endpoint path changes; drop the `properties.id == clientId` rule. |
| **Capabilities endpoint** | `POST` then `PUT` `/clients/{clientId}/capabilities[/{deviceId}]`; k8s‑style `{apiVersion,kind,properties}` | `PUT` (+ `DELETE`) `/capabilities/{deviceId}`; LinkML body (no `apiVersion`/`kind` envelope); hierarchical `{deviceId}` | Rewrite capability scenarios + bodies + the mock. |
| **Desired state** | `GET /clients/{clientId}/deployments` | `GET /deployments` | Path only; body shape essentially same (`manifestVersion`, `bundle`, `deployments`). |
| **Bundle / YAML** | `GET /clients/{clientId}/bundles/{digest}`, `…/deployments/{id}/{digest}` | `GET /bundles/{digest}`, `GET /deployments/{id}/{digest}` | Path only; digest/ETag/immutable rules unchanged. |
| **Status** | `POST /clients/{clientId}/deployments/{id}/status`; `{apiVersion,kind,deploymentId,status,components}`; states `installed/installing/uninstalling/failed` | `POST /deployments/{id}/status`; add `adoptedManifestVersion`, optional `deviceId`; states add `removing/removed` | Rewrite status scenarios + the mock's checks. |
| **Auth failure** | `401` "signature verification failed" / `403` untrusted cert | **TLS handshake failure** (connection refused/reset) or `403` `not-authorized` / `wfm-client-relationship-retired` for policy | Negative auth scenarios become "connection fails" assertions + a `403` policy test; the `401` cases are deleted. |
| **Error body** | ad‑hoc `{"Error": "..."}` | RFC 9457 `application/problem+json` (+ `retryable`, `backoffStrategy`, `errors[]`) | Every validation that checks `Error` must check `type`/`title`/`status`; new `retryable` assertions. |
| **Digest / ETag / 304 / immutable** | Present | **Unchanged** (MI‑004/006/007/009/015/025/027/030/031/034/035 all still apply) | Keep these checks; just move to the new paths. |
| **HTTP/1.1, port 443** | Required | **Unchanged** | Keep. |
| **`wait` (AD‑001)** | `properties.wait`, default true; `packageLocation` | `properties.wait`, default true, + `timeout`; component uses `repository` (`oci://…`) + `revision` (semver) | Update the multi‑component fixture (`packageLocation` → `repository`+`revision`). |
| **Application Description envelope** | `apiVersion: margo.org/v1-alpha1`, `kind: ApplicationDescription` required | `apiVersion: v1`; **`kind` removed from the schema entirely** (confirmed against `feature/miaf`'s package validator, which no longer checks it) | Fix `margo.yaml` in our Application Registry fixture (`wfm-supplier/fixtures/margo-ctt-package/`) and re‑push to Harbor. |

**CR‑ID fate (WFM Management Interface):**

- **Still valid, new transport:** MI‑004, 006, 007, 009, 015, 023, 025, 026, 027,
  030, 031, 034, 035, 038 (caching/digest/manifest‑version contract).
- **Obsolete (RFC 9421 specifics):** MI‑021 (read `Signature-Input`/`Signature`/
  `Content-Digest`), MI‑022 (reconstruct signature base string), MI‑028 (support
  all signature algorithms *for message signatures*), MI‑029 (`created` timestamp
  replay), MI‑013 (`created` validity window), MI‑041 (`401` on signature
  failure). These are replaced by **mTLS/SVID** requirements from the identity
  spec (new CR‑IDs will be `MARGO-…-IDENTITY-*` / `MARGO-…-TLS-*` — the identity
  pages don't carry CR‑IDs yet).
- **Changed shape:** MI‑011 (look up client cert) → now "validate SVID + apply
  accepted‑client policy"; MI‑020 (WFM creates URL‑safe client ID) → gone (no
  server‑assigned ID); MI‑002 (client ID is UUIDv4…) → replaced by `wfm-client-id`
  charset rules.
- **New requirement areas with no CR‑IDs yet:** discovery document, Trust Bundle
  format + refresh/replace/fail‑closed, SVID leaf constraints, SPIFFE‑ID exact‑match
  recognition, mTLS version/cipher, connection‑lifetime re‑validation, proxy rules.

---

## 7. Conformance‑suite migration plan

> **Revised 2026‑09‑17** after the manager's decision: **we do not build or
> run our own MIS.** Trust‑bundle handling is a one‑time step at the start of
> a suite run, not a per‑test‑case concern, and not a component we own.
> Everything below reflects that — it is a smaller plan than the first draft.

### 7.1 New capabilities the suite needs

| # | Capability | Where | Notes |
|---|---|---|---|
| A | **mTLS HTTP client** presenting an X.509‑SVID, validating the server SVID's SPIFFE ID | `run_wfm_scenarios.js` (Node `https` `cert`/`key`/`ca` + a SAN‑URI check), device‑supplier Go mock's `tls.Config` | Replaces the whole RFC 9421 block. Biggest single change, but it's a transport swap, not new architecture. |
| B | **One‑time trust‑bundle fetch at suite startup (WFM‑supplier persona)** — `GET /.well-known/margo` then `GET <trustBundleUri>` against the *real* WFM supplier's declared MIS, cached in memory for the whole run | `run_wfm_scenarios.js`, a small pre‑flight step before any scenario runs | **Not** built/owned by us — we are a client of *their* MIS. No mocking. |
| C | **A 2‑endpoint discovery stub at suite startup (device‑supplier persona)** — our harness serves `/.well-known/margo` + `/.well-known/spiffe/bundle.json` once, so the real device‑under‑test can bootstrap trust in us | extend the existing `cmd/device-supplier` mock with two static routes | This is a static file responder, not a real MIS — no minting, no dynamic state. |
| D | **Our own SVID** (cert + key) to present in every mTLS handshake, on both personas | manually minted once (out‑of‑band, e.g. via whatever MIS instance is reachable for provisioning), stored as a fixture like today's `certs/` dir | One‑time manual setup, same posture as existing cert fixtures — not suite‑automated. |
| E | **Negative SVID/TLS fixtures** — expired SVID, wrong trust‑domain, `cA=true` leaf, 2 URI SANs, chains to a non‑bundle anchor | fixtures + mock knobs | Mirrors the existing `negative_fixtures.go` pattern (named modes, verdict in Go). Keep this list short initially — expand only if real implementations need it. |
| F | **`application/problem+json` assertions** — operator on `type`/`status`/`retryable` | `run_wfm_scenarios.js` `validate()` (new `problem_type_is`, `is_retryable` ops) | Replaces `{"Error"}` checks. |
| G | **Accepted‑client‑policy test** — a valid SVID that is *not* on the allow‑list → `403` `not-authorized`; and `wfm-client-relationship-retired` | scenario + mock policy config | New, but small — one positive + one negative scenario. |
| H | **Live allow‑list revocation test** (device‑supplier persona) — start accepted, remove the WFM's SPIFFE ID from the device's `authorized.json` mid‑run, confirm the *very next* request is rejected with no agent restart | scenario for `cmd/device-supplier`‑style testing once that mock exists | Confirmed real/testable behavior per Part 5.8 (hot‑reload + keep‑alives‑disabled) — not previously buildable. |

**Explicitly out of scope for now** (deliberately not building, to avoid
over‑engineering a suite that's about to change again):
- **No MIS server of our own** — confirmed by the manager; both personas only
  ever *consume* trust‑bundle material, never issue it.
  See Part 5.7 for why this doesn't block anything above.
- **No mint automation** — it's a manual, offline, Unix‑socket‑only CLI step
  in the reference implementation; treat it as a setup script, not a feature.
- **No trust‑bundle rotation/refresh testing** — the reference MIS itself
  hardcodes its bundle's sequence number today (no rotation to test against
  yet); revisit once that's real (see Part 8, open question 6).
- **Device‑supplier "call the device" style tests** — the device has no
  inbound API in the new flow (Part 5.7); this whole persona's test design
  needs its own decision once the dev team's device‑side MIAF work lands
  (see Part 8, open question 5) — not something to build reflexively now.

### 7.2 Scenario file rewrites

- **`testcases/wfm-core/postman_collection.json`** — Postman **cannot do mTLS
  cert presentation or a TLS‑handshake‑failure assertion**. Options:
  1. keep Postman for the request/response‑shape checks (paths, bodies, ETag,
     `problem+json`, status codes) and configure the client cert at the Newman/
     collection level (`--ssl-client-cert`), **or**
  2. move the WFM persona fully onto the declarative `run_wfm_scenarios.js` runner
     (which we control) and retire Newman for this persona.
  Given mTLS + SPIFFE‑ID validation + Trust‑Bundle refresh logic, **option 2 is
  cleaner** — but that's a call to make with the manager (it changes the "author
  in Postman" story).
- **New declarative groups:**
  - `wfm-identity` — discovery doc, Trust Bundle fetch/refresh/replace/fail‑closed,
    server‑SVID validation, mTLS version, connection‑lifetime.
  - `wfm-core-v2` — the 6 management endpoints on the new paths, `problem+json`,
    caching, `adoptedManifestVersion`.
  - `wfm-policy` — accepted‑client‑policy + relationship‑retired.
- **`testcases/device-conformance/test-scenarios.json`** — the device persona
  keeps its declarative format; rewrite: drop `flex-onboarding` cert‑upload steps,
  add "device fetches + refreshes Trust Bundle", "device presents its SVID",
  "device validates the WFM SVID's SPIFFE ID", "device rejects an empty‑anchor
  bundle"; move all endpoints to the new paths; add `adoptedManifestVersion` and
  `removing`/`removed` to status steps; update the multi‑component fixture
  (`repository`+`revision`).

### 7.3 Runner code changes (minimise, reuse — same discipline as before)

- `run_wfm_scenarios.js`: **remove** `signRequest`, `prepareContentDigest`,
  `computeKeyId`, `tamper_signature`, `skip_signing`, the `keyid` plumbing.
  **Add** `tls: { cert, key, ca }` from a cert dir, a `serverSvidSpiffeId` check
  hook, `problem_type_is` / `is_retryable` operators, and a **one‑time**
  pre‑flight step (run once before the scenario loop starts, not per test)
  that fetches the trust bundle from the WFM supplier's declared MIS and
  caches it for the run. Net: likely **fewer** lines than today — no MIS mock
  to write here, since we're only ever a client.
- `cmd/device-supplier/main.go`: swap `http.Server` for one with
  `TLSConfig{ ClientAuth: RequireAndVerifyClientCert, ClientCAs: <bundle> }` and a
  `VerifyPeerCertificate` that enforces the SPIFFE‑ID shape + accepted‑client
  policy; drop the RFC 9421 verifier; keep `negative_fixtures.go` /
  `manifest_rules.go` (digest logic is unchanged) and add `svid_fixtures.go` in
  the same named‑mode style. **Add** the 2 discovery‑stub routes
  (`/.well-known/margo`, `/.well-known/spiffe/bundle.json`) as plain static
  handlers, served once at mock startup — not a real MIS implementation.
- `run_tests.go` (our simulated device): present a client SVID; validate the mock
  WFM's SVID; drop signing.

### 7.4 Order of work (proposed)

1. **Our own SVID fixture** (D) — mint once, out of band, store like today's
   `certs/` dir.
2. **`run_wfm_scenarios.js` → mTLS transport swap** (A) — the highest‑value,
   lowest‑risk change: it doesn't depend on anyone else's work landing.
3. **One‑time trust‑bundle fetch pre‑flight** (B) — small addition on top of 2.
4. **Fix the Application Package fixture** (`apiVersion`/`kind`, see §6) and
   re‑push to Harbor.
5. **`device‑supplier` mock → mTLS + discovery stub** (A, C) — do this once the
   dev team's device‑side MIAF work is far enough along to test against (see
   Part 8, open question 5); no reason to build it blind before then.
6. **Rewrite device `test-scenarios.json`** to the new paths, once step 5 exists.
7. **Rewrite / re‑platform the WFM consolidated file** (decision needed: Postman
   vs all‑declarative — mTLS cert presentation isn't something Postman/Newman
   can do natively).
8. **New `wfm-identity` / `wfm-policy` groups** (E, F, G above) — small,
   additive, can happen any time after step 3.
9. **CR‑ID re‑map** in both spreadsheets — mark the RFC 9421 MIs obsolete, add the
   identity/TLS rows (even without official CR‑IDs, track them as
   `MARGO-WFM-IDENTITY-00x (proposed)`).

---

## 8. Open questions to resolve with the team / spec authors

1. ~~**Do we need to build our own MIS?**~~ **Resolved 2026‑09‑17 (manager):**
   No. WFM‑supplier testing does a one‑time trust‑bundle fetch from the real
   WFM's declared MIS; device‑supplier testing hosts only the 2 discovery
   endpoints as a static stub, once per suite run. See Part 5.7 / 7.1.
2. **Do the identity spec pages get CR‑IDs?** The Management Interface pages have
   `MARGO-WFM-MANAGEMENTINTERFACE-0xx`; the identity pages currently don't. We need
   IDs to track coverage. (Raise on Discourse / the spec repo.)
3. **Is `v1alpha2` / the `/v1alpha2/margo` prefix still a thing**, or does the new
   API sit at `/api/v1` with no version segment? Symphony currently serves
   `/v1alpha2/margo/api/v1/...`.
4. **Postman vs all‑declarative for the WFM persona** (§7.2) — mTLS + Trust‑Bundle
   logic pushes toward all‑declarative. Manager's call.
5. **How do we get a pre‑provisioned deployment** for the still‑blocked MIs
   (MI‑005/014/024/025/033/038)? The multi‑component group already needs an
   operator to assign an app; MIAF doesn't change that.
6. **Does Symphony implement any of MIAF yet?** If not, the whole `wfm-core-v2` /
   `wfm-identity` suite will be red against it — useful as a gap report, but we
   should confirm the target before investing. **Status:** per the manager, the
   dev team is building the *device* side of MIAF first and will pick up
   Symphony after — we're intentionally waiting on that before building the
   device‑supplier half of this (see §7.4 step 5).
7. **Trust‑anchor rotation testing** — worth a scenario (publish 2 anchors →
   client accepts both → retire old), or defer as operator‑playbook territory?
   The reference MIS itself doesn't support rotation yet (hardcoded sequence
   number), so there's nothing real to test against today.
8. **Where does our one SVID actually get minted from?** The reference `mis
   mint x509` CLI needs a running MIS host with Unix‑socket access — do we
   get one issued from an existing shared MIS instance (e.g. the one on
   Pulkit's VM), or does someone stand up a throwaway MIS just long enough to
   mint our fixture cert once? Either way it's a one‑time action, not
   suite infrastructure — just needs an owner.
9. **Is the `refreshHint` units bug (Part 5.8) real?** Worth a quick check
   against the vendored `go-spiffe` library before we build anything that
   depends on the actual refresh‑interval value — if it's real, it's a bug
   report to file against `margo/sandbox`, not something for us to work
   around silently.

---

## Appendix A — key spec URLs (`pre-draft` branch)

| Topic | URL |
|---|---|
| **Reference implementation (Part 5.7 source)** | `https://github.com/margo/sandbox/tree/feature/miaf` — the actual MIS, device‑agent, and WFM reference code this doc's Part 5.7 was cross‑checked against |
| WFM Client Onboarding (the flow) | `general_website_content/pre-draft/system-design/concepts/workload-fleet-managers/wfm-client-onboarding.md` |
| Identity & Trust (plain‑language) | `.../concepts/identity/identity-and-trust.md` |
| MIAF framework | `specification/pre-draft/system-design/specification/identity/identity-framework.md` |
| WFM Identity Profile | `.../identity/wfm-identity-profile.md` |
| SVIDs (crypto + validation) | `.../identity/svids.md` |
| TLS requirements (bootstrap, session) | `.../identity/tls-requirements.md` |
| Trust Bundle & Discovery | `.../identity/trust-bundle-and-discovery.md` |
| Identity Lifecycle & Playbooks | `.../identity/identity-lifecycle.md` |
| Trust Bundle API (OpenAPI) | `.../identity/trust-bundle-api-1.0.0-rc.2.yaml` |
| API Requirements & Security | `.../margo-management-interface/api-requirements-and-security.md` |
| Device Capabilities API | `.../margo-management-interface/device-capabilities.md` |
| Deployment Status API | `.../margo-management-interface/deployment-status.md` |
| Workload Management API (OpenAPI) | `.../margo-management-interface/workload-management-api-1.0.0-rc.2.yaml` |
| Desired‑state schema (LinkML) | `specification/pre-draft/src/specification/margo-management-interface/desired-state.linkml.yaml` |
| Problem Types registry | `.../specification/problem-types.md` |
| Device Requirements | `.../specification/margo-devices/device-requirements.md` |
| Technical Lexicon | `general_website_content/pre-draft/system-design/personas-definitions/technical-lexicon.mdx` |

All under `https://raw.githubusercontent.com/margo/<repo>/…`.
