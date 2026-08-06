# Margo Conformance Test Suite — Demo Guide

---

## 1. The one-paragraph version

Margo is a spec that defines how a **device** (an edge machine running workloads) and a **WFM** (Workload Fleet Manager — the cloud/control-plane software that manages many devices) are supposed to talk to each other over HTTPS. This suite lets a vendor who built *either side* prove their implementation follows that spec correctly — without needing the other side's real system. We give them a trustworthy **mock** stand-in for whichever side they didn't build, run a battery of test calls, and hand back a pass/fail report.

Two vendor situations, two personas:

| Persona | The vendor brought... | We provide... |
|---|---|---|
| **Device Supplier** | A real device / device-agent | A mock WFM server to test it against |
| **WFM Supplier** | A real WFM (e.g. Eclipse Symphony) | A mock device that fires test calls at it |

(There's also an **Application Supplier** persona for validating application packages.)

---

## 2. The conversation both personas are testing

Regardless of which side is "real" and which is "mock," every test is checking the same underlying conversation, because that conversation *is* the Margo spec:

1. **Get the WFM's root certificate** — the device downloads the WFM's CA cert so it knows who it's talking to (`GET /onboarding/certificate`).
2. **Onboard** — the device introduces itself by sending its own certificate; the WFM registers it and hands back a `clientId` (`POST /onboarding`). Think of it like a new employee showing ID at security on day one — everything after this uses that badge.
3. **Report capabilities** — the device tells the WFM what it can run (hardware, OS, resources) (`POST/PUT /clients/{clientId}/capabilities`).
4. **Fetch desired state** — the device asks "what am I supposed to be running?" (`GET /clients/{clientId}/deployments`).
5. **Download the actual deployment content** — the device pulls the application bundle / deployment manifest referenced by the desired state (`GET .../bundles/{digest}`, `GET .../deployments/{id}/{digest}`).
6. **Report status** — the device reports back whether the deployment succeeded, failed, or is progressing (`POST /clients/{clientId}/deployments/{id}/status`).

From step 3 onward, every request is **cryptographically signed** (RFC 9421 HTTP Message Signatures) using the device's certificate — so the WFM can verify it's really talking to the device it onboarded, not an impostor.

Every test in this suite is really just: *"send one of these calls, in some circumstance (normal, or deliberately broken), and check the response is exactly what the spec says it should be."*

---

## 3. Device Supplier persona

**Who's real, who's mock:** the vendor's **device** is real. **mock WFM server** (a small Go program) plays the role of the WFM.

**How they connect:** the real device is pointed at the mock WFM's URL (`https://<host>:3001/v1alpha2/margo`) instead of a real WFM, and it runs through the exact 6-step conversation above. The mock WFM answers with real, spec-correct responses — including the *correct error* when the device does something wrong (missing signature → 401, bad certificate → 400, unknown/untrusted certificate → 403, malformed request body → 422, and so on).

**What we're testing:** does the device's own client software correctly speak the protocol — right endpoints, right signing, right handling of both success *and* error responses?

**How we're testing it, two ways:**
- **Fixed order** — the classic path: onboarding, then capabilities, then desired state, then deployment, then status-back, always in that order. This matches how a simple, well-behaved device would naturally proceed.
- **Flex (random) order** — after onboarding (which always has to be first — you can't report capabilities before you exist), the remaining calls fire in a *random* relative order each run. This proves the mock WFM doesn't secretly assume a fixed sequence, because a real device implementation is free to call capabilities, desired-state, and status-reporting in whatever order makes sense to it. Only onboarding is a hard prerequisite.

**Groups — what they're for:** a *group* is just a named, curated bundle of test cases (e.g. `bronze`, `gold`, `flex-order`) so you can run a targeted subset instead of everything at once — useful for tiering (bronze/silver/gold conformance levels), or isolating a particular test style. A group is one folder containing a `group.json` (name, version, description, which test-case IDs belong to it) plus the test-case files it references.

**Commands:**

```bash
cd conformance

# 1) Create / prepare test data and groups
./conformance.sh
  → 2  (Device Supplier)
  → create a new group, or select an existing one

# (non-interactive equivalent)
./conformance.sh device "/path/to/test-scenarios.json"

# 2) Run the tests for a group
./run-tests.sh
  → 2  (Device Supplier)
  → pick a group (e.g. bronze, flex-order)
  → point it at the mock WFM URL (or the vendor's real WFM, if testing the reverse direction)

# (non-interactive equivalent)
./run-tests.sh device bronze

# Report lands in:
conformance/Runner/device-supplier/conformance-report-<timestamp>.html
```

---

## 4. WFM Supplier persona

**Who's real, who's mock:** the vendor's **WFM** is real (e.g. an Eclipse Symphony instance). Our **mock device** plays the role of the device.

**What "mock device" actually is:** it's not a separate server — it's a script (`run_wfm_scenarios.js`) that *acts as* a device. It has its own certificate and private key, computes real RFC 9421 signatures, and fires the same 6-step conversation — but now *we're* the one initiating calls, against the vendor's real WFM URL.

**How they connect:** you give the script the vendor's real WFM URL (e.g. `https://symphony.machine:8082/v1alpha2/margo`), and it runs through onboarding → capabilities → desired state → bundle/manifest download → status report, exactly like a real device would, including deliberately-broken variants (skip the signature, send a bad digest, send garbage certificate data, ask for a non-existent deployment) to confirm the WFM rejects those correctly too.

**What we're testing:** does the vendor's WFM implementation correctly accept valid device onboarding, validate certificates and signatures, serve capabilities/desired-state/bundles/manifests correctly, record status reports, and return the *right* error code for the *right* reason?

**How we're testing it:** the test data (a Postman-style collection) documents, for every endpoint, both a success example and every documented error example. The script turns each documented example into an actual HTTP call engineered to trigger that exact condition, sends it, and compares the real response against what's expected. Some checks intentionally allow more than one correct answer (e.g. both 400 and 401 are reasonable ways for different implementations to say "bad request") — that flexibility is explicit and documented, never silently applied.

**Groups — what they're for:** same idea as the device side — a named bundle of specific test-case IDs from a Postman collection (e.g. `bronze grp`, `silver`, `diamond`) so you can run a targeted subset.

**Commands:**

```bash
cd conformance

# 1) Create / prepare test data and groups
./conformance.sh
  → 1  (WFM Supplier)
  → 1  OpenAPI spec based (generate tests from an OpenAPI spec), or
  → 2  Functional tests (group-based, from a Postman collection)
  → create a new group, or select an existing one

# (non-interactive equivalent)
./conformance.sh wfm openapi /path/to/openapi.yaml

# 2) Run the tests for a group, against the vendor's real WFM
./run-tests.sh
  → 1  (WFM Supplier)
  → pick a group (e.g. "bronze grp")
  → enter the vendor's real WFM Server URL

# (non-interactive equivalent)
./run-tests.sh wfm "bronze grp" https://symphony.machine:8082/v1alpha2/margo

# Report lands in:
conformance/Runner/wfm-supplier/wfm-scenario-report-<group>_<timestamp>.html
```

---

## 5. Why should a client trust this suite?

1. **We test our own mock against itself first.** Before we ever point this at a client's real system, our own simulated device runs the *entire* fixed-order suite against our own mock WFM, and it has to pass 100%. That proves our reference implementation of "correct spec behavior" actually is correct and self-consistent, on both sides we control.
2. **Every expected outcome traces back to the spec, not to what's convenient.** Each test step encodes an exact expected status code and response shape taken from the Margo API definition — not something invented ad hoc.
3. **We test failure paths, not just the happy path.** A big share of the test set is deliberately-broken requests: missing signature, wrong certificate, tampered digest, malformed body. A system that only ever gets the happy path right isn't actually spec-conformant — correctly *rejecting* bad input is just as important, and this suite checks that explicitly on both personas.
4. **The exact same rules apply whether we're testing our own mock or a client's real system.** There's no special leniency mode for "friendly" tests — the same pass/fail logic, the same expected codes, run either way. When we find a gap (like we did earlier today — a certificate validation gap in our own mock's onboarding), we fix it the same way we'd expect a vendor to fix theirs.
5. **Reports are generated from real evidence, not manual sign-off.** Every row in the report is a real HTTP request that was actually sent and a real response that was actually received, with the expected vs. actual value shown side by side — it's an auditable trail, not a checklist someone filled in by hand.

---

## 6. Command cheat sheet

```bash
cd conformance

# Create test data / groups (CLI #1)
./conformance.sh                              # interactive
./conformance.sh device <scenarios.json>      # device supplier, direct
./conformance.sh wfm openapi <spec-path>      # wfm supplier, direct

# Run tests (CLI #2)
./run-tests.sh                                # interactive
./run-tests.sh device <group-name>            # device supplier, direct
./run-tests.sh wfm <group-name> <wfm-url>     # wfm supplier, direct
./run-tests.sh help                           # full built-in help text
```

---

## 7. Anticipated questions (FAQ)

**"What's a mock server / mock device, in plain terms?"**
A stand-in that behaves exactly like the real thing is supposed to, per spec, so you can test the other side in isolation. Mock WFM = a fake-but-correct cloud manager. Mock device = a script that behaves exactly like a real, well-behaved (and occasionally deliberately misbehaving) device.

**"Why do I need groups — why not just run everything?"**
You often want a smaller, targeted run — e.g. only the tests relevant to a conformance tier, or only the tests for one feature you just changed — instead of the full suite every time. Groups also let you organize by intent (fixed-order vs. flex-order, bronze vs. gold tier, etc.).

**"What happens if a real device or WFM fails a test?"**
The report shows exactly which step failed, what was expected, and what was actually returned — enough detail for the vendor to go fix the specific gap, not just "something's wrong."

**"What's the difference between the fixed-order and flex-order tests?"**
Fixed-order assumes the classic, predictable call sequence. Flex-order proves the system under test doesn't *require* that exact sequence beyond "onboarding must come first" — because real devices may legitimately call things in a different order.

**"Is this testing security too, or just functionality?"**
Both. Signature verification, certificate validation, and rejection of untrusted/malformed input are core parts of the test set.

**"Do I need my own real device or WFM to see this work?"**
No — for the demo, both personas can be run entirely against our own mock implementations, so you can see the full flow, the groups, and the reports without needing a live third-party system.

**"How long does a run take?"**
Seconds to few minutes, depending on the group size — this is direct HTTP calls, not a slow end-to-end environment spin-up.

**"What do I actually get at the end?"**
A self-contained HTML report per run: pass/fail counts, every step's expected vs. actual result, and (for WFM Supplier runs) which real WFM URL was tested — saved under `Runner/<persona>/`.

**"Can I test just one thing, like only onboarding?"**
Yes — both CLIs support running a single group, and the device-supplier runner also supports filtering to a single scenario or even a single step via flags, for focused debugging.
