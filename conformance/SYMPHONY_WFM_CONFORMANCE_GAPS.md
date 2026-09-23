# Symphony WFM — Conformance Gaps

**Suite:** wfm-supplier `core` group (`testcases/wfm-core/postman_collection.json`)
**Target WFM:** `https://symphony.machine:8082/v1alpha2/margo`
**Run date:** 2026-09-09 &nbsp;·&nbsp; **Result:** 10 / 31 pass, 21 fail
**Cross-checked:** the same failures reproduce from the device-supplier suite pointed at Symphony, so they are the WFM's, not our client's.

Of the 21 failing steps: **18 are genuine Symphony conformance gaps** (below), **3 are weak test cases on our side** (last section).

---

## Genuine Symphony gaps

| # | Area | Test case | Expected | Symphony returned | Symphony's own message |
|---|------|-----------|:--------:|:-----------------:|------------------------|
| 1 | **Onboarding — cert validation** | Onboard with a malformed certificate (`f9e10c20`) | `400` | **`201`** | *(client onboarded successfully)* |
| 2 | **Capabilities — enum validation** | Report capabilities with an invalid `supportedDeploymentTypes` value (`39ee0019`) | `422` | **`201`** | `"Device capabilities reported successfully"` |
| 3 | **Capabilities — enum validation** | Report capabilities with an invalid `supportedRuntimes` value (`288e3214`) | `422` | **`201`** | `"Device capabilities reported successfully"` |
| 4 | **Capabilities — semantic validation** | POST capabilities with a semantically-invalid body (`a3f7146c`) | `422` | **`201`** | `"Device capabilities reported successfully"` |
| 5 | **Capabilities — semantic validation** | PUT (update) capabilities with a semantically-invalid body (`8ec05b96`) | `422` | **`201`** | `"Device capabilities updated successfully"` |
| 6 | **Capabilities — Content-Digest** | POST capabilities with **no** `Content-Digest` header (`f206fb46`) | `400` | **`201`** | `"Device capabilities reported successfully"` |
| 7 | **Capabilities — Content-Digest** | PUT capabilities with **no** `Content-Digest` header (`eb9e178c`) | `400` | **`201`** | `"Device capabilities updated successfully"` |
| 8 | **Deployment status — semantic validation** | POST status with an invalid `state` value (`6e674338`) | `422` | **`400`** | `"invalid state: __invalid_state__"` |
| 9 | **Desired state — default content negotiation** | GET `/deployments` with **no `Accept` header** (`6da61c30`) | `200` + manifest media type | **`500`** | `"Unknown State: 406: The accept header should be application/vnd.margo.manifest.v1+json"` |
| 10 | **Desired state — content negotiation** | GET `/deployments` with an **unsupported `Accept`** header (`f54748a1`) | `406` | **`500`** | `"Unknown State: 406: The accept header should be application/vnd.margo.manifest.v1+json"` |
| 11 | **Auth — signature failure code** | POST capabilities with an invalid signature (`5fc47546`) | `401` | **`400`** | `"htmsig.Verify: signature verification failed … invalid ECDSA signature"` |
| 12 | **Auth — signature failure code** | PUT capabilities with an invalid signature (`7b600483`) | `401` | **`400`** | `"htmsig.Verify: signature verification failed … invalid ECDSA signature"` |
| 13 | **Auth — signature failure code** | POST deployment status with an invalid signature (`387e5c5b`) | `401` | **`400`** | `"htmsig.Verify: signature verification failed … invalid ECDSA signature"` |
| 14 | **Auth — untrusted certificate** | POST capabilities signed by a never-onboarded cert (`066c0155`) | `403` / `401` | **`400`** | `"htmsig.Verify: signature verification failed …"` |
| 15 | **Auth — untrusted certificate** | PUT capabilities signed by a never-onboarded cert (`6fb8f5bb`) | `403` / `401` | **`400`** | `"htmsig.Verify: signature verification failed …"` |
| 16 | **Auth — untrusted certificate** | POST deployment status signed by a never-onboarded cert (`292d8e91`) | `403` / `401` | **`400`** | `"htmsig.Verify: signature verification failed …"` |
| 17 | **Caching — conditional GET** | GET `/deployments` with `If-None-Match` (`17a2b787`) | `304 Not Modified` | **`200`** + full manifest | *(no 304 support)* |
| 18 | **Caching — manifest marked immutable** | GET `/deployments` — Cache-Control on the manifest response (`2bd8b6a4`) | must **not** contain `immutable` | **`public, max-age=31536000, immutable`** | *(header value)* |

### Grouped by root cause

**A. Symphony does not validate request bodies — accepts them with `201`**  (#1–7)
Invalid certificates, invalid capability enum values, invalid manifest semantics, and requests missing the mandatory `Content-Digest` header all succeed. The WFM performs essentially no input validation on onboarding / capabilities. This is the most significant cluster — a non‑conformant or buggy device gets silently accepted.

**B. Wrong HTTP status codes for errors Symphony *does* detect**  (#8, #11–16)
Symphony correctly identifies the problem (bad state value, bad signature, unknown signer) but answers with `400 Bad Request` instead of the spec‑mandated code:
- semantic body error → should be `422`, is `400` (#8)
- signature verification failure → should be `401` (spec MI‑041), is `400` (#11–13)
- untrusted / unknown signing certificate → should be `401` or `403`, is `400` (#14–16)

**C. Content negotiation crashes with `500`**  (#9, #10)
`GET /deployments` returns HTTP **500** both when the `Accept` header is omitted (should default to the manifest media type and return `200`) and when it carries an unsupported value (should return `406 Not Acceptable`). Symphony's error body literally says "406" but the status line is 500 — a device that doesn't send exactly the right `Accept` header gets a server error.

**D. Caching contract not honoured**  (#17, #18)
- **#17** — `If-None-Match` is ignored; the WFM always returns the full manifest instead of `304`. Devices poll `/deployments` on an interval and rely on `304` to skip re-downloading an unchanged manifest — this defeats that design.
- **#18** — the manifest response carries `Cache-Control: public, max-age=31536000, immutable`. MI-035 forbids this: a manifest changes over time (its `manifestVersion` increments), so it must never be cached as permanently unchanging. That header belongs on content-addressed bundles, not manifests.

---

## Not Symphony gaps — weak test cases on our side (excluded from the count above)

| Test case | Expected | Got | Why it's not a real finding |
|-----------|:--------:|:---:|-----------------------------|
| Bundle-by-digest — "Representation not modified" (`5755e4a7`) | `304` | `404` | Uses an all-zeros digest against a client with **no bundle assigned**. `404` is the correct response. Needs a real onboarded client that has a deployment/bundle to test meaningfully. |
| Bundle-by-digest — "Invalid request" (`a7d2c754`) | `400` | `404` | Same all-zeros digest — it's a well-formed but non-existent digest, so `404` is correct. |
| Onboard with an unknown/fresh certificate (`e89bca4b`) | `403` | `201` | Onboarding is *where* a device registers its certificate — accepting a new one is arguable spec behaviour. The "client rejected (403)" example really targets a **blocklisted** cert, which needs server-side blocklist state we can't set from the client. |

---

## Notes for the discussion

- All 17 gaps are **reproducible** and backed by Symphony's own error responses (quoted above).
- The signature / cert / content-negotiation clusters (B, C) are "right detection, wrong response" — likely small fixes in Symphony's error-mapping layer.
- Cluster A (no body validation) is the substantive one and probably a larger effort.
- Suite changes made so we could attribute these cleanly (all committed / pending review):
  - 422 tests now send a valid current-schema body with one bad enum (not the old placeholder body)
  - signature-failure tests send a present-but-wrong signature (not a missing one)
  - capabilities endpoint rule now also matches the `…/capabilities` path (no `{deviceId}`) used by the baseline collection
