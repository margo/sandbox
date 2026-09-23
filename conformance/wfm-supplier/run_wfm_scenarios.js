#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');
const { execFileSync } = require('child_process');

function usage() {
  console.error(
    'Usage: node run_wfm_scenarios.js <base-url> <scenarios.json> <report.html> <cert-dir> [group-name] [group-version]\n' +
    '   or: node run_wfm_scenarios.js --curl <METHOD> <endpoint> --base-url <url> --cert-dir <dir> ' +
    '[--body <json>] [--header "Name: value"]... [--unsigned]'
  );
  process.exit(2);
}

// --curl debug mode: sign and fire ONE ad-hoc request, print the equivalent curl
// command plus the raw response — for manually poking at a requirement on the CLI,
// the way you'd use Postman. Reuses signRequest/prepareContentDigest/request/
// injectCertificate below (hoisted function declarations) instead of a second,
// separate signing implementation. See runCurlMode() further down.
const isCurlMode = process.argv[2] === '--curl';

let baseUrlArg, scenariosFile, reportFile, certDir, groupName, groupVersion, curlArgs;
if (isCurlMode) {
  curlArgs = parseCurlArgs(process.argv.slice(3));
  baseUrlArg = curlArgs.baseUrl;
  certDir = curlArgs.certDir;
  if (!baseUrlArg || !certDir || !curlArgs.method || !curlArgs.endpoint) usage();
} else {
  [baseUrlArg, scenariosFile, reportFile, certDir, groupName, groupVersion] =
    process.argv.slice(2);
  if (!baseUrlArg || !scenariosFile || !reportFile || !certDir) usage();
}

const baseUrl = baseUrlArg.replace(/\/+$/, '');
const privateKeyPath = path.join(certDir, 'device.key');
const deviceCertPath = path.join(certDir, 'device-cert.pem');
const caCertPath = path.join(certDir, 'ca-cert.pem');

// CTT Margo Version — the Margo spec version this conformance tool validates against
function readCttMargoVersion() {
  try {
    const specText = fs.readFileSync(path.join(__dirname, 'spec.yaml'), 'utf8');
    const match = specText.match(/^\s*version:\s*(\S+)/m);
    return match ? match[1] : 'unknown';
  } catch {
    return 'unknown';
  }
}
const cttMargoVersion = readCttMargoVersion();

// Compute SHA-256 hex thumbprint of PKIX DER public key — matches ComputeKeyIDFromPrivateKeyPEM
// in shared-lib/crypto/keyid.go so the WFM can correlate the keyid to the registered cert.
function computeKeyId(privateKeyPem) {
  try {
    const privObj = crypto.createPrivateKey(privateKeyPem);
    const pubDer = crypto.createPublicKey(privObj).export({ format: 'der', type: 'spki' });
    return crypto.createHash('sha256').update(pubDer).digest('hex');
  } catch {
    return 'device-key'; // fallback for unexpected key formats
  }
}

let privateKey = fs.readFileSync(privateKeyPath, 'utf8');
let deviceCertificate = fs.readFileSync(deviceCertPath, 'utf8');
let deviceCertificateBase64 = Buffer.from(deviceCertificate).toString('base64');
const caCertificate = fs.existsSync(caCertPath) ? fs.readFileSync(caCertPath) : undefined;

let keyid = computeKeyId(privateKey);

// MIAF (mTLS) identity — loaded only if present, so scenarios/environments that
// don't have one yet are completely unaffected. Same certDir as the RFC 9421
// identity above; separate files because it's a different credential (an
// X.509-SVID with a SPIFFE URI SAN), not a replacement for the old one.
// See wfm-supplier/fixtures/miaf/README.md for how to generate/replace it.
const svidCertPath = path.join(certDir, 'svid-cert.pem');
const svidKeyPath  = path.join(certDir, 'svid-key.pem');
const svidCaPath   = path.join(certDir, 'svid-ca.pem');
const miafIdentity = (fs.existsSync(svidCertPath) && fs.existsSync(svidKeyPath))
  ? {
      cert: fs.readFileSync(svidCertPath, 'utf8'),
      key: fs.readFileSync(svidKeyPath, 'utf8'),
      ca: fs.existsSync(svidCaPath) ? fs.readFileSync(svidCaPath, 'utf8') : caCertificate,
    }
  : null;

// Identity/certs are loaded above exactly as the normal run does — curl mode exits
// here, before anything below that depends on a scenarios file (a bare top-level
// `return` is valid because Node wraps this file in a function).
if (isCurlMode) {
  runCurlMode(curlArgs)
    .then((failed) => process.exit(failed ? 1 : 0))
    .catch((err) => { console.error(err.stack || err); process.exit(1); });
  return;
}

// ─────────────────────────────────────────────────────────────────────────────
// Postman collection format detection and conversion
// ─────────────────────────────────────────────────────────────────────────────

function isPostmanCollection(data) {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false;
  if (!Array.isArray(data.item)) return false;
  // Standard Postman v2.1 / portman generated: info.schema points to getpostman.com
  if (data.info && typeof data.info.schema === 'string' && data.info.schema.includes('getpostman.com')) return true;
  // Postman export with info but without schema (older/different formats)
  if (data.info && typeof data.info === 'object' && (data.info.name || data.info._postman_id)) return true;
  // Portman/openapi-to-postman style: metadata lives in _ key instead of info
  if (data._ && typeof data._ === 'object' && data._.postman_id) return true;
  return false;
}

// Rename :paramName segments to {contextVar} with semantic disambiguation.
// The Postman spec uses ":digest" for both bundle download and deployment YAML —
// these need different context variables because they come from different response fields.
function postmanSegToContextVar(seg, precedingPath) {
  if (!seg.startsWith(':')) return seg;
  const varName = seg.slice(1);
  if (varName === 'digest') {
    const joined = precedingPath.join('/');
    if (joined.endsWith('bundles')) return '{bundleDigest}';
    return '{deploymentDigest}';
  }
  return `{${varName}}`;
}

function postmanPathToEndpoint(pathArr) {
  return '/' + pathArr.map((seg, i) => postmanSegToContextVar(seg, pathArr.slice(0, i))).join('/');
}

// Rules keyed by "METHOD /endpoint" (with {vars} substituted in).
// body:        replace request body entirely
// bodyMerge:   shallow-merge top-level fields into the parsed Postman body
// bodyNested:  set nested fields (dot-notation keys like "properties.id")
// extract_context, validations, skip_signing: override defaults
// Current spec (docs.margo.org/specification/margo-management-interface/device-capabilities):
// no "roles" field (removed, not renamed), and "resources" is gone — cpus/memory/storage/
// peripherals/interfaces are flat under properties, cpus is an array (was singular "cpu"),
// plus three new fields: otelCollector, supportedRuntimes, supportedDeploymentTypes.
const DEVICE_CAPABILITIES_BODY = {
  apiVersion: 'device.margo.org/v1alpha1',
  kind: 'DeviceCapabilitiesManifest',
  properties: {
    id: '{deviceId}',
    vendor: 'Acme Corp',
    modelNumber: 'ACM-XYZ',
    serialNumber: 'SN-12345',
    cpus: [{ cores: 4, architecture: 'arm64' }],
    memory: '16Gi',
    storage: '256Gi',
    peripherals: [],
    interfaces: [{ type: 'ethernet' }],
    otelCollector: false,
    supportedRuntimes: ['oci'],
    supportedDeploymentTypes: ['helm', 'compose'],
  },
};

const POSTMAN_ENDPOINT_RULES = {
  'GET /api/v1/onboarding/certificate': {
    skip_signing: true,
    validations: [{ field: 'certificate', operation: 'is_string' }],
  },
  'POST /api/v1/onboarding': {
    body: {
      apiVersion: 'onboarding.margo.org/v1alpha1',
      kind: 'OnboardingRequest',
      certificate: './certs/device-cert.pem',
    },
    // deviceId: this suite models one device per client, so the device shares the
    // client's identity. Spec allows deviceId to differ from clientId (e.g. a gateway
    // fronting multiple child devices); revisit if/when a multi-device scenario is added.
    extract_context: { clientId: 'clientId', deviceId: 'clientId' },
    validations: [{ field: 'clientId', operation: 'is_string' }],
  },
  // Spec path is /clients/{clientId}/capabilities/{deviceId} — verified against Symphony
  // directly (both the old no-deviceId path and this one return 201; the spec's is correct
  // and matches the currently-published API version).
  'POST /api/v1/clients/{clientId}/capabilities/{deviceId}': {
    body: DEVICE_CAPABILITIES_BODY,
  },
  'PUT /api/v1/clients/{clientId}/capabilities/{deviceId}': {
    body: DEVICE_CAPABILITIES_BODY,
  },
  // Same endpoint without the {deviceId} segment — the shape the baseline
  // (user1) Postman collection uses. Without this, capability items fall back to
  // the Portman placeholder body and get a spurious 400 "invalid API version".
  'POST /api/v1/clients/{clientId}/capabilities': {
    body: DEVICE_CAPABILITIES_BODY,
  },
  'PUT /api/v1/clients/{clientId}/capabilities': {
    body: DEVICE_CAPABILITIES_BODY,
  },
  'GET /api/v1/clients/{clientId}/deployments': {
    extract_context: {
      deploymentId: 'deployments.0.deploymentId',
      bundleDigest: 'bundle.digest',
      deploymentDigest: 'deployments.0.digest',
    },
    validations: [
      { field: 'manifestVersion', operation: 'is_number' },
      // MI-009: bundle present & null when deployments is empty.
      { operation: 'bundle_null_when_no_deployments' },
      // MI-031: correct bundle media type when a bundle is present.
      { operation: 'bundle_media_type' },
      // MI-034: the ETag digest must be bare — quoted per HTTP ETag syntax and
      // nothing else (no weak prefix, no whitespace, no extra characters).
      { field: '_headers.etag', operation: 'matches_regex', value: '^"sha256:[0-9a-f]{64}"$' },
      // MI-035: a manifest response MUST NOT be marked immutable.
      { field: '_headers.cache-control', operation: 'not_contains', value: 'immutable' },
      // MI-015: the ETag MUST be a strong validator = sha256 of the exact body.
      { operation: 'etag_is_body_digest' },
    ],
  },
  'GET /api/v1/clients/{clientId}/bundles/{bundleDigest}': {
    // Bundle endpoint: passes when a bundle exists (200); 404 is expected when no deployments configured
    accepted_statuses: [200, 404],
  },
  'GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{deploymentDigest}': {
    // Deployment YAML: passes when a deployment exists; 404 is expected when none configured
    accepted_statuses: [200, 404],
  },
  'POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status': {
    bodyMerge: {
      apiVersion: 'deployment.margo.org/v1alpha1',
      kind: 'DeploymentStatusManifest',
      deploymentId: '{deploymentId}',
    },
    // Status endpoint: passes when a deployment exists; 400/404 expected when none configured
    accepted_statuses: [200, 400, 404],
  },
};

// semanticErrorBody returns a structurally valid current-schema request body for
// `endpoint` with exactly one deliberate semantic violation (a bad enum value),
// so a 422 test reaches the server's semantic-validation layer instead of being
// rejected earlier as a malformed payload. Returns null for endpoints we don't
// have a known-good body shape for (caller falls back to the Postman body).
function semanticErrorBody(endpoint) {
  if (endpoint.includes('/capabilities')) {
    const b = JSON.parse(JSON.stringify(DEVICE_CAPABILITIES_BODY));
    b.properties.supportedDeploymentTypes = ['__invalid_deployment_type__'];
    return b;
  }
  if (endpoint.endsWith('/status')) {
    return {
      apiVersion: 'deployment.margo.org/v1alpha1',
      kind: 'DeploymentStatusManifest',
      deploymentId: '{deploymentId}',
      status: { state: '__invalid_state__' },
      components: [{ name: 'component-1', state: 'installed' }],
    };
  }
  return null;
}

// ─────────────────────────────────────────────────────────────────────────────
// Error-triggering rules for each response-example sub-test
// ─────────────────────────────────────────────────────────────────────────────

// Returns flags that tell runStep how to produce the request that should cause
// the expected error status.
//
//   skip_signing        → omit Signature-Input / Signature / Content-Digest (triggers 400/401)
//   fresh_cert          → sign with a brand-new, never-onboarded certificate (triggers 401/403)
//   use_placeholder_body→ keep Postman's Lorem-Ipsum body rather than applying ENDPOINT_RULES
//                         (triggers 422 for semantic-validation failures)
//   bad_certificate     → replace onboarding body.certificate with a plaintext string (→ 400)
//   wrong_accept        → send Accept: text/plain instead of the media type the server expects (→ 406)
//   add_if_none_match   → add If-None-Match: * to get a potential 304 (server may still return 200)
//   accepted_statuses   → array of HTTP codes that are all acceptable for this step
function deriveErrorBehavior(resp) {
  const code = resp.code ?? 0;
  const name = (resp.name || '').toLowerCase();

  if (code === 304) {
    // "Not Modified" — send If-None-Match; the spec documents 304 as the sole valid
    // response for a matching conditional GET. No 200/404 fallback: neither is a
    // documented outcome here, so accepting them would hide a server that doesn't
    // honor If-None-Match at all.
    return { add_if_none_match: true };
  }
  if (code === 406) {
    // "Not Acceptable" — request an unsupported Accept header; the spec documents only
    // 406 for this case. A 500 means the server errored instead of correctly rejecting
    // the request — that's a real defect, not an acceptable alternative.
    return { wrong_accept: true };
  }
  if (/content.?digest/i.test(name)) {
    // "Missing or invalid content-digest" — omit only the Content-Digest header (not the
    // signature). Spec documents 400 for this specific case; 401 belongs to the separate
    // signature-failure test below.
    return { skip_content_digest: true };
  }
  if (/signature.*fail|signature.*verif/i.test(name) || code === 401) {
    // "Signature verification failed" — send a fully-formed request (valid
    // Signature-Input + Content-Digest) whose Signature bytes are wrong, so the
    // server's verifier runs and rejects it. `skip_signing` would instead omit
    // the headers entirely, which is "missing signature" — a different case that
    // a strict server answers with 400, not the 401 this test documents.
    return { tamper_signature: true };
  }
  if (/invalid.*cert.*format|cert.*format.*invalid|cert.*format.*struct/i.test(name)) {
    // skip_signing: prevents 409 "already registered" when the device keyid is already onboarded.
    // Spec documents only 400 for invalid certificate format on onboarding.
    return { bad_certificate: true, skip_signing: true };
  }
  if (/not.*trusted|revoked|rejected/i.test(name) || code === 403) {
    // "Certificate not trusted / revoked" — sign with a fresh, never-onboarded
    // certificate so the WFM sees an identity it has no trust relationship with.
    // A conformant WFM may answer 403 (recognised but not trusted) or 401 (could
    // not authenticate the caller) here — the strict 403-vs-revoked distinction
    // needs server-side revocation state this client-only test can't create — so
    // both codes are accepted. Anything else (400, 2xx) is still a failure.
    return { fresh_cert: true, accepted_statuses: [401, 403] };
  }
  if (/semantic.*error|body.*semantic|request body includes/i.test(name) || code === 422) {
    // Spec documents 422 for a semantic body error; 400 belongs to content-digest, a different test.
    // We send a STRUCTURALLY VALID current-schema body with exactly one semantic
    // violation (a bad enum value) — the Portman placeholder body has an invalid
    // apiVersion and old-schema fields, so a strict server rejects it with 400
    // ("invalid API version …") before it ever reaches semantic validation, which
    // is not what this test is checking.
    return { semantic_error: true };
  }
  // Any other documented error (404 CONTEXT_FALLBACKS, or anything else): hold it to its
  // own documented code — no blanket fallback (e.g. 301 isn't documented anywhere in spec).
  return {};
}

// Build one runStep descriptor from a single response example inside a Postman item.
// Success responses (2xx) apply the full POSTMAN_ENDPOINT_RULES; error responses
// apply deriveErrorBehavior so the runner intentionally triggers the expected failure.
function deriveStepFromResponse(parentItem, resp) {
  const req = parentItem.request;
  if (!req) return null;

  const pathArr = req.url?.path || [];
  const endpoint = postmanPathToEndpoint(pathArr);
  const method = (req.method || 'GET').toUpperCase();
  const ruleKey = `${method} ${endpoint}`;
  const rules = POSTMAN_ENDPOINT_RULES[ruleKey] || {};

  const expected_status = resp.code ?? 200;
  const is_success = expected_status >= 200 && expected_status < 300;

  const headers = {};
  for (const h of req.header || []) {
    if (!h.disabled) headers[h.key] = h.value;
  }

  const errorBehavior = is_success ? {} : deriveErrorBehavior(resp);

  const skip_signing = errorBehavior.skip_signing ?? (rules.skip_signing ?? false);
  const skip_content_digest = errorBehavior.skip_content_digest ?? false;
  const fresh_cert   = errorBehavior.fresh_cert ?? false;
  const tamper_signature = errorBehavior.tamper_signature ?? false;

  // Derive the request body
  let request_body = null;
  if (req.body?.raw) {
    try { request_body = JSON.parse(req.body.raw); } catch (_) {}
  }

  if (is_success) {
    // Happy-path: apply ENDPOINT_RULES (real Margo API values)
    if (rules.body) {
      request_body = rules.body;
    } else {
      if (rules.bodyMerge && request_body) request_body = { ...request_body, ...rules.bodyMerge };
      if (rules.bodyNested && request_body) request_body = applyBodyNested(request_body, rules.bodyNested);
    }
  } else if (errorBehavior.bad_certificate) {
    // 400 "Invalid certificate format" — send a plaintext string as the certificate field.
    // Unique per run: a fixed literal gets remembered as "already registered" by a real,
    // persistent WFM (like Symphony), turning this into a false 409 on the next run instead
    // of the intended 400.
    request_body = {
      ...(rules.body || {}),
      certificate: `INVALID_NOT_A_PEM_CERTIFICATE-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    };
  } else if (errorBehavior.semantic_error) {
    // 422 "Semantic error" — a valid current-schema body with one bad enum value.
    request_body = semanticErrorBody(endpoint) || request_body;
  } else if (errorBehavior.wrong_accept) {
    headers['Accept'] = 'text/plain';
  } else if (errorBehavior.add_if_none_match) {
    // 304 "Not Modified" — send a wildcard ETag; server decides whether to honour it
    headers['If-None-Match'] = '*';
  } else if (errorBehavior.fresh_cert) {
    // 403 "Certificate not trusted" — body uses the same structure but signing uses a fresh cert.
    // The cert placeholder in the body (if any) will be replaced by injectCertificate with the
    // temporarily-substituted deviceCertificateBase64 for the fresh cert.
    if (rules.body) {
      request_body = rules.body;
    } else {
      if (rules.bodyMerge && request_body) request_body = { ...request_body, ...rules.bodyMerge };
      if (rules.bodyNested && request_body) request_body = applyBodyNested(request_body, rules.bodyNested);
    }
  } else {
    // Generic error (401 skip_signing, 404, etc.): use rules body if available
    if (rules.body) {
      request_body = rules.body;
    } else {
      if (rules.bodyMerge && request_body) request_body = { ...request_body, ...rules.bodyMerge };
      if (rules.bodyNested && request_body) request_body = applyBodyNested(request_body, rules.bodyNested);
    }
  }

  // Context extraction only for success steps (no point extracting from error responses)
  const extract_context = is_success ? (rules.extract_context || {}) : {};
  const validations    = is_success ? (rules.validations ?? derivePostmanValidations(parentItem)) : [];

  // accepted_statuses: for success steps use rules (e.g. bundle 200 also accepts 404 when no data),
  // for error steps use the error behavior derivation
  const accepted_statuses = is_success
    ? (rules.accepted_statuses || undefined)
    : errorBehavior.accepted_statuses;

  // Conformance-requirement IDs: a response example may narrow them (e.g. only the
  // error example maps to a "must be rejected" CR-ID); otherwise inherit the item's.
  const crIds = resp.crIds || resp.crids || parentItem.crIds || parentItem.crids || [];

  return {
    id: resp.id,
    name: `${parentItem.name} — ${resp.name || resp.status}`,
    crIds,
    method,
    endpoint,
    headers,
    ...(request_body !== null ? { request_body } : {}),
    expected_status,
    ...(accepted_statuses ? { accepted_statuses } : {}),
    validations,
    extract_context,
    skip_signing,
    skip_content_digest,
    fresh_cert,
    tamper_signature,
  };
}

// Derive validations from Postman pm.test script content.
function derivePostmanValidations(item) {
  const validations = [];
  const scripts = (item.event || [])
    .filter((e) => e.listen === 'test')
    .flatMap((e) => e.script?.exec || [])
    .join('\n');

  // Content-Type header check: ...headers.get("Content-Type")...to.include("value")
  const ctMatch = scripts.match(/headers\.get\(["']Content-Type["']\)[^"']*["']([^"']+)["']\)/);
  if (ctMatch) {
    validations.push({ field: '_headers.content-type', operation: 'contains', value: ctMatch[1] });
  }

  // Top-level schema fields: extract simple string/number properties
  const schemaMatch = scripts.match(/const schema = (\{[\s\S]*?\})\s*\n\n/);
  if (schemaMatch) {
    try {
      const schema = JSON.parse(schemaMatch[1]);
      for (const [key, def] of Object.entries(schema.properties || {})) {
        if (!Array.isArray(def.type)) {
          if (def.type === 'string') validations.push({ field: key, operation: 'is_string' });
          else if (def.type === 'number') validations.push({ field: key, operation: 'is_number' });
        }
      }
    } catch (_) { /* large or complex schemas may not parse — skip */ }
  }

  return validations;
}

function applyBodyNested(body, nestedRules) {
  if (!body || typeof body !== 'object') return body;
  const result = { ...body };
  for (const [dotPath, val] of Object.entries(nestedRules)) {
    const parts = dotPath.split('.');
    let obj = result;
    for (let i = 0; i < parts.length - 1; i++) {
      if (obj && typeof obj === 'object') obj = obj[parts[i]];
      else { obj = null; break; }
    }
    if (obj && typeof obj === 'object') obj[parts[parts.length - 1]] = val;
  }
  return result;
}

function convertPostmanItem(item) {
  const req = item.request;
  if (!req) return null;

  const pathArr = req.url?.path || [];
  const endpoint = postmanPathToEndpoint(pathArr);
  const method = (req.method || 'GET').toUpperCase();
  const ruleKey = `${method} ${endpoint}`;
  const rules = POSTMAN_ENDPOINT_RULES[ruleKey] || {};

  // Headers: only non-disabled entries
  const headers = {};
  for (const h of req.header || []) {
    if (!h.disabled) headers[h.key] = h.value;
  }

  // Body: parse Postman raw JSON, then apply rules
  let request_body = null;
  if (req.body?.raw) {
    try { request_body = JSON.parse(req.body.raw); } catch (_) {}
  }
  if (rules.body) {
    request_body = rules.body;
  } else {
    if (rules.bodyMerge && request_body) request_body = { ...request_body, ...rules.bodyMerge };
    if (rules.bodyNested && request_body) request_body = applyBodyNested(request_body, rules.bodyNested);
  }

  const expected_status = item.response?.[0]?.code ?? 200;
  const validations = rules.validations ?? derivePostmanValidations(item);
  const extract_context = rules.extract_context || {};
  const skip_signing = rules.skip_signing ?? false;

  return {
    id: item.id,
    name: item.name,
    crIds: item.crIds || item.crids || [],
    method,
    endpoint,
    headers,
    ...(request_body !== null ? { request_body } : {}),
    expected_status,
    validations,
    extract_context,
    skip_signing,
  };
}

function parsePostmanCollection(collection) {
  const successSteps = [];
  const errorSteps   = [];

  for (const item of (collection.item || [])) {
    const responses = item.response || [];

    if (responses.length === 0) {
      // No response examples: create a single step from the item itself (legacy path)
      const step = convertPostmanItem(item);
      if (step) successSteps.push(step);
      continue;
    }

    // Expand to one step per response example.
    // Success (2xx) steps first so context (clientId, etc.) is populated
    // before error steps try to reference it in URLs.
    for (const resp of responses) {
      const step = deriveStepFromResponse(item, resp);
      if (!step) continue;
      const is_success = step.expected_status >= 200 && step.expected_status < 300;
      (is_success ? successSteps : errorSteps).push(step);
    }
  }

  const info = collection.info || {};
  const description =
    (typeof info.description === 'object' ? info.description.content : info.description) ||
    collection._?.description ||
    'Auto-converted from Postman collection';

  return [
    {
      id: info._postman_id || collection._?.postman_id || 'postman-collection',
      name: info.name || 'Postman Collection',
      description,
      steps: [...successSteps, ...errorSteps],
    },
  ];
}

// ─────────────────────────────────────────────────────────────────────────────

const rawData = JSON.parse(fs.readFileSync(scenariosFile, 'utf8'));
const scenarios = isPostmanCollection(rawData) ? parsePostmanCollection(rawData) : rawData;
let context = {};
const results = [];
const scenarioResults = [];

function regenerateCertificate() {
  const { execSync } = require('child_process');
  const tempDeviceId = `device-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
  execSync(`openssl ecparam -name prime256v1 -genkey -noout -out "${privateKeyPath}"`);
  execSync(
    `openssl req -new -x509 -days 365 -key "${privateKeyPath}" -out "${deviceCertPath}"` +
      ` -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=${tempDeviceId}"`
  );
  privateKey = fs.readFileSync(privateKeyPath, 'utf8');
  deviceCertificate = fs.readFileSync(deviceCertPath, 'utf8');
  deviceCertificateBase64 = Buffer.from(deviceCertificate).toString('base64');
  keyid = computeKeyId(privateKey);
}

// Fallback values for context vars that are empty or not yet set.
// These are used in URL path segments to produce a valid (but non-existent) URL
// instead of a double-slash like /deployments// which triggers redirects.
// A well-formed fake ID ensures the WFM returns a proper 404 rather than 301.
const CONTEXT_FALLBACKS = {
  deploymentId:    'deployment-none-00000000',
  deploymentDigest:'sha256:0000000000000000000000000000000000000000000000000000000000000000',
  bundleDigest:    'sha256:0000000000000000000000000000000000000000000000000000000000000000',
};

function substitute(value) {
  if (typeof value === 'string') {
    return value.replace(/\{([^}]+)\}/g, (_, key) => {
      const val = context[key];
      if (val !== undefined && val !== null && val !== '') return String(val);
      return CONTEXT_FALLBACKS[key] ?? '';
    });
  }
  if (Array.isArray(value)) return value.map(substitute);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, substitute(v)]));
  }
  return value;
}

function getField(source, field) {
  if (field === '_body') return source._body;
  const parts = field.split('.');
  let current = source;
  let inHeaders = false;
  for (const part of parts) {
    if (current == null) return undefined;
    if (part === '_headers') { current = current._headers; inHeaders = true; continue; }
    if (inHeaders) {
      // Node.js lowercases all response header names; do case-insensitive lookup
      // so the JSON can use conventional capitalization like "ETag" or "Content-Type".
      const lower = part.toLowerCase();
      const key = Object.keys(current || {}).find((k) => k.toLowerCase() === lower);
      current = current ? current[key ?? part] : undefined;
      continue;
    }
    if (/^\d+$/.test(part)) { current = current[Number(part)]; continue; }
    current = current[part];
  }
  return current;
}

// Signs the request following RFC 9421 HTTP Message Signatures.
// Components signed: @method, @target-uri, @authority (always) + content-digest (when body present).
// @authority is required by the WFM verifier (shared-lib/crypto verifier.go WithComponents).
// Content-Digest is computed and stored in headers before this function is called.
function signRequest(method, url, headers, bodyText) {
  const parsedUrl = new URL(url);
  const authority = parsedUrl.host; // hostname:port

  const components = ['@method', '@target-uri', '@authority'];
  const lines = [
    `"@method": ${method.toUpperCase()}`,
    `"@target-uri": ${url}`,
    `"@authority": ${authority}`,
  ];

  // Include content-digest in signature only when it has a non-empty value.
  if (bodyText && headers['Content-Digest']) {
    components.push('content-digest');
    lines.push(`"content-digest": ${headers['Content-Digest']}`);
  }

  const created = Math.floor(Date.now() / 1000);
  const signatureParams =
    `(${components.map((c) => `"${c}"`).join(' ')});created=${created};keyid="${keyid}"`;
  lines.push(`"@signature-params": ${signatureParams}`);

  // Use ieee-p1363 dsaEncoding so ECDSA signatures are emitted as raw r||s (64 bytes for P-256)
  // rather than DER/ASN.1. The Go verifier (dsig.UnpackECDSASignature) checks len == keySize*2
  // and rejects DER-encoded signatures.
  const signatureBytes = crypto.sign('sha256', Buffer.from(lines.join('\n')), {
    key: privateKey,
    dsaEncoding: 'ieee-p1363',
  });

  headers['Signature-Input'] = `sig1=${signatureParams}`;
  headers.Signature = `sig1=:${signatureBytes.toString('base64')}:`;
}

// Prepares Content-Digest for any request that carries a body.
// If the step has already set Content-Digest (even to "") in its headers, that value is
// preserved so negative tests can exercise the empty-digest path.
function prepareContentDigest(headers, bodyText) {
  if (!bodyText) return;
  if (Object.prototype.hasOwnProperty.call(headers, 'Content-Digest')) return;
  const digest = crypto.createHash('sha256').update(bodyText).digest('base64');
  headers['Content-Digest'] = `sha-256=:${digest}:`;
}

// `mtls` is optional: { cert, key, ca } to present a client certificate for MIAF's
// mutual-TLS handshake. Omitted for every existing (RFC 9421 / server-TLS-only)
// call site — this only changes behavior for callers that opt in.
function request(method, url, headers, bodyText, mtls) {
  return new Promise((resolve) => {
    const parsedUrl = new URL(url);
    const options = {
      method,
      hostname: parsedUrl.hostname,
      port: parsedUrl.port || 443,
      path: parsedUrl.pathname + parsedUrl.search,
      headers,
      ca: (mtls && mtls.ca) || caCertificate,
      rejectUnauthorized: false,
      timeout: 30000,
    };
    if (mtls) {
      options.cert = mtls.cert;
      options.key = mtls.key;
    }

    const req = https.request(options, (res) => {
      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => {
        const body = Buffer.concat(chunks).toString('utf8');
        resolve({ status: res.statusCode, headers: res.headers, body });
      });
    });

    req.on('timeout', () => {
      req.destroy(new Error('request timed out after 30s'));
    });
    req.on('error', (err) => {
      resolve({ status: 0, headers: {}, body: '', transportError: err.message });
    });

    if (bodyText) req.write(bodyText);
    req.end();
  });
}

// ─────────────────────────────────────────────────────────────────────────────
// --curl debug mode
// ─────────────────────────────────────────────────────────────────────────────

// Parses: --curl <METHOD> <endpoint> --base-url <url> --cert-dir <dir>
//         [--body <json>] [--header "Name: value"]... [--unsigned]
function parseCurlArgs(argv) {
  const out = { method: argv[0], endpoint: argv[1], headers: {}, unsigned: false };
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--base-url') out.baseUrl = argv[++i];
    else if (a === '--cert-dir') out.certDir = argv[++i];
    else if (a === '--body') out.body = argv[++i];
    else if (a === '--header') {
      const [name, ...rest] = (argv[++i] || '').split(':');
      if (name) out.headers[name.trim()] = rest.join(':').trim();
    } else if (a === '--unsigned') out.unsigned = true;
  }
  return out;
}

// Prints a curl command equivalent to the request this process is about to make,
// then makes that exact request and prints the raw response — signing it with
// the SAME signRequest()/prepareContentDigest() the scenario runner uses, so what
// you see here always matches real runner behaviour (no second signing path to
// keep in sync). Returns true if the request failed at the transport level.
async function runCurlMode(args) {
  const url = `${baseUrl}${args.endpoint}`;
  const headers = { ...args.headers };
  let bodyText = '';
  if (args.body !== undefined) {
    // Reuse injectCertificate's "./certs/device-cert.pem" marker when the body is
    // valid JSON; an intentionally-malformed body (negative testing) is sent as-is.
    try {
      const parsed = injectCertificate(JSON.parse(args.body), {});
      bodyText = JSON.stringify(parsed);
    } catch {
      bodyText = args.body;
    }
    headers['Content-Type'] = headers['Content-Type'] || 'application/json';
  }

  if (bodyText) prepareContentDigest(headers, bodyText);
  if (!args.unsigned) signRequest(args.method, url, headers, bodyText);

  const quote = (s) => `'${String(s).replace(/'/g, `'"'"'`)}'`;
  const curlLines = [`curl -sS -k -i -X ${args.method.toUpperCase()} ${quote(url)}`];
  for (const [k, v] of Object.entries(headers)) curlLines.push(`  -H ${quote(`${k}: ${v}`)}`);
  if (bodyText) curlLines.push(`  --data ${quote(bodyText)}`);
  console.log('\n── Equivalent curl command ──────────────────────────────────');
  console.log(curlLines.join(' \\\n'));

  console.log('\n── Sending request ──────────────────────────────────────────');
  const response = await request(args.method.toUpperCase(), url, headers, bodyText);
  if (response.transportError) {
    console.log(`✗ transport error: ${response.transportError}`);
    return true;
  }
  console.log(`HTTP ${response.status}`);
  for (const [k, v] of Object.entries(response.headers)) console.log(`${k}: ${v}`);
  console.log('');
  try {
    console.log(JSON.stringify(JSON.parse(response.body), null, 2));
  } catch {
    console.log(response.body);
  }
  console.log('');
  return false;
}

function validate(responseSource, validation) {
  // Field-less validations (the bundle_* semantic checks below) carry no `field`.
  const actual =
    validation.field == null ? undefined : getField(responseSource, validation.field);
  const expected = substitute(validation.value);

  switch (validation.operation) {
    case 'exists':
      return actual !== undefined && actual !== null ? '' : `${validation.field} is missing`;
    case 'is_string':
      return typeof actual === 'string' ? '' : `${validation.field} is not a string`;
    case 'is_number':
      return typeof actual === 'number' ? '' : `${validation.field} is not a number`;
    case 'is_array':
      return Array.isArray(actual) ? '' : `${validation.field} is not an array`;
    case 'not_empty':
      return actual !== undefined && actual !== null && String(actual).length > 0
        ? ''
        : `${validation.field} is empty`;
    case 'equals':
      return actual === expected
        ? ''
        : `${validation.field} expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`;
    case 'contains':
      return String(actual ?? '').includes(String(expected))
        ? ''
        : `${validation.field} does not contain ${JSON.stringify(expected)}`;
    case 'array_length_gte':
      return Array.isArray(actual) && actual.length >= Number(expected)
        ? ''
        : `${validation.field} expected length >= ${expected}, got ${Array.isArray(actual) ? actual.length : typeof actual}`;
    case 'array_length_equals':
      return Array.isArray(actual) && actual.length === Number(expected)
        ? ''
        : `${validation.field} expected length === ${expected}, got ${Array.isArray(actual) ? actual.length : typeof actual}`;
    case 'one_of':
      return Array.isArray(expected) && expected.includes(actual)
        ? ''
        : `${validation.field} expected one of ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`;
    case 'matches_regex':
      // Uses the raw pattern, not the {context}-substituted `expected` — a
      // regex quantifier like {64} would otherwise be mistaken for a
      // context-variable placeholder by substitute() and silently stripped.
      return new RegExp(validation.value).test(String(actual ?? ''))
        ? ''
        : `${validation.field} value ${JSON.stringify(actual)} does not match pattern ${validation.value}`;
    case 'greater_than':
      return Number(actual) > Number(expected)
        ? ''
        : `${validation.field} expected > ${expected}, got ${actual}`;
    case 'not_contains':
      // Passes when the value is absent or does not include the substring.
      return !String(actual ?? '').includes(String(expected))
        ? ''
        : `${validation.field} must not contain ${JSON.stringify(expected)} (got ${JSON.stringify(actual)})`;
    case 'etag_is_body_digest': {
      // MI-015: the State Manifest ETag MUST be a strong validator computed as
      // sha256 of the exact serialized JSON response body.
      const rawEtag = getField(responseSource, '_headers.etag');
      if (rawEtag == null) return 'ETag header is missing';
      const body = responseSource._body ?? '';
      const bodyDigest = 'sha256:' + crypto.createHash('sha256').update(body, 'utf8').digest('hex');
      const normalised = String(rawEtag).replace(/^W\//, '').replace(/^"|"$/g, '');
      return normalised === bodyDigest
        ? ''
        : `ETag ${JSON.stringify(rawEtag)} is not sha256 of the response body (expected ${bodyDigest}) (MI-015)`;
    }
    case 'body_sha256_equals': {
      // MI-025: the deployment/bundle digest advertised in the desired-state
      // manifest MUST be sha256 of the exact bytes served at the content-addressed
      // URL. `validation.value` is the expected digest (usually a {context} var
      // such as {deploymentDigest}); compared hex-to-hex, ignoring an optional
      // "sha256:" prefix on either side.
      const body = responseSource._body ?? '';
      const actualHex = crypto.createHash('sha256').update(body, 'utf8').digest('hex');
      const expectedHex = String(expected ?? '').replace(/^sha256:/, '').toLowerCase();
      if (!expectedHex) return `${validation.field ?? 'digest'} expected value is empty`;
      return actualHex === expectedHex
        ? ''
        : `served body sha256 (${actualHex}) does not match the manifest digest (${expectedHex}) (MI-025)`;
    }
    case 'has_application_description_layer': {
      // AR-002 (the one part every package MUST have, regardless of vendor):
      // a layer with the fixed Application Description media type MUST exist.
      // Ref-agnostic — doesn't need to know the vendor's file name.
      const layers = getField(responseSource, 'layers') || [];
      const found = layers.some((l) => l.mediaType === 'application/vnd.margo.app.description.v1+yaml');
      return found
        ? ''
        : 'no layer has mediaType "application/vnd.margo.app.description.v1+yaml" for the Application Description';
    }
    case 'layer_media_types_are_margo_specific': {
      // AR-004/011: every layer a package DOES ship MUST carry a Margo-specific
      // vendor media type, never a generic one (application/octet-stream etc.).
      // Ref-agnostic by design — doesn't assume which resource files a given
      // vendor's package includes or what they're named, only that whatever is
      // present is correctly typed.
      const layers = getField(responseSource, 'layers') || [];
      const generic = layers
        .filter((l) => !String(l.mediaType || '').startsWith('application/vnd.margo.app.'))
        .map((l) => `${(l.annotations && l.annotations['org.opencontainers.image.title']) || l.digest}: "${l.mediaType}"`);
      return generic.length === 0
        ? ''
        : `layer(s) with a non-Margo-specific mediaType: ${generic.join(', ')}`;
    }
    case 'bundle_null_when_no_deployments': {
      // MI-009: when there are zero deployments the `bundle` field MUST be
      // present with the value null (not omitted). JSON.parse keeps the key, so
      // `bundle === null` is true only for an explicit null; `undefined` (the
      // field absent) fails the check as it should.
      const deployments = getField(responseSource, 'deployments');
      const bundle = getField(responseSource, 'bundle');
      if (Array.isArray(deployments) && deployments.length === 0) {
        return bundle === null
          ? ''
          : 'bundle must be present and null when the deployments array is empty (MI-009)';
      }
      return '';
    }
    case 'bundle_media_type': {
      // MI-031: bundle.mediaType MUST be application/vnd.margo.bundle.v1+tar+gzip.
      // Skipped on a zero-deployment manifest, where bundle is null.
      const bundle = getField(responseSource, 'bundle');
      if (bundle == null) return '';
      return bundle.mediaType === 'application/vnd.margo.bundle.v1+tar+gzip'
        ? ''
        : `bundle.mediaType expected "application/vnd.margo.bundle.v1+tar+gzip", got ${JSON.stringify(bundle.mediaType)} (MI-031)`;
    }
    default:
      return `unsupported validation operation: ${validation.operation}`;
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function parseBody(body) {
  if (!body) return {};
  try {
    return JSON.parse(body);
  } catch {
    return { _body: body };
  }
}

function injectCertificate(body, step) {
  if (!body || step.skip_certificate_injection) return body;
  if (body.certificate === './certs/device-cert.pem') {
    return { ...body, certificate: deviceCertificateBase64 };
  }
  return body;
}

// Returns the context keys that were newly set (so caller can display them).
function extractContext(responseSource, extractors) {
  const extracted = {};
  for (const [key, field] of Object.entries(extractors || {})) {
    const value = getField(responseSource, field);
    if (value !== undefined && value !== null) {
      context[key] = value;
      extracted[key] = value;
    }
  }
  return extracted;
}

function failureSummary(response, assertionFailures) {
  if (response.transportError) return response.transportError;
  if (assertionFailures.length > 0) return assertionFailures.join('; ');
  return '';
}

// ─────────────────────────────────────────────────────────────────────────────
// Console output helpers
// ─────────────────────────────────────────────────────────────────────────────

const COL = 72;
const THICK_LINE = '═'.repeat(COL);
const THIN_LINE  = '─'.repeat(COL);

const HTTP_STATUS_TEXT = {
  200: 'OK', 201: 'Created', 204: 'No Content', 304: 'Not Modified',
  400: 'Bad Request', 401: 'Unauthorized', 403: 'Forbidden',
  404: 'Not Found', 406: 'Not Acceptable', 409: 'Conflict',
  422: 'Unprocessable Entity', 500: 'Internal Server Error', 503: 'Service Unavailable',
};
function httpLabel(code) {
  return code > 0
    ? `${code} ${HTTP_STATUS_TEXT[code] || ''}`.trim()
    : 'NO RESPONSE (network error)';
}

function truncate(str, max) {
  const s = String(str ?? '');
  return s.length > max ? s.slice(0, max - 1) + '…' : s;
}

function printBanner() {
  const grp  = groupName    ? `Group: ${groupName}${groupVersion ? ` (v${groupVersion})` : ''}` : 'WFM Scenario Test';
  const wfm  = `WFM:   ${baseUrl}`;
  console.log('\n' + THICK_LINE);
  console.log(` Margo WFM Conformance Test Runner`);
  console.log(` ${grp}`);
  console.log(` ${wfm}`);
  console.log(THICK_LINE);
}

function printScenarioHeader(scenario, index, total) {
  console.log('\n' + THIN_LINE);
  console.log(` SCENARIO ${index} of ${total}  ·  ${scenario.name}`);
  if (scenario.description) {
    // Word-wrap description at COL-2 chars
    const words = scenario.description.split(' ');
    let line = ' ';
    for (const w of words) {
      if (line.length + w.length + 1 > COL - 1) { console.log(line); line = ` ${w}`; }
      else { line += (line === ' ' ? '' : ' ') + w; }
    }
    if (line.trim()) console.log(line);
  }
  console.log(THIN_LINE);
}

function signingLabel(step, bodyText) {
  if (step.oras)              return '[oras / registry — no signing]';
  if (step.mtls)              return '[mTLS · X.509-SVID]';
  if (step.skip_signing)      return '[unsigned]';
  if (step.tamper_signature)  return '[bad-signature]';
  if (step.fresh_cert)        return '[fresh-cert · signed]';
  if (bodyText) return '[signed · content-digest]';
  return '[signed]';
}

function printStepResult(step, result, newContext, bodyText) {
  const pass = result.passed;
  const icon = pass ? '✓' : '✗';
  const tag  = pass ? 'PASS' : 'FAIL';
  const sig  = signingLabel(step, bodyText);

  console.log('');
  console.log(`  [${step.id}]  ${step.name}`);
  console.log(`   ▶  ${result.method.padEnd(6)} ${result.endpoint}  ${sig}`);
  if (result.crIds && result.crIds.length) console.log(`   ⚙  ${result.crIds.join(', ')}`);

  if (pass) {
    console.log(`   ${icon}  ${tag}  ${httpLabel(result.actual)}`);
    // Show any newly extracted context values (e.g., clientId from onboarding)
    for (const [key, val] of Object.entries(newContext)) {
      const display = truncate(String(val), 60);
      console.log(`        ↳ ${key} = "${display}"`);
    }
  } else {
    console.log(`   ${icon}  ${tag}  expected ${httpLabel(result.expected)}  ·  got ${httpLabel(result.actual)}`);
    const failures = result.reason.split('; ');
    for (const f of failures) {
      if (f.trim()) console.log(`        • ${f.trim()}`);
    }
  }
}

function printScenarioSummary(passed, total) {
  const status = passed === total ? '✓ all passed' : `✗ ${total - passed} failed`;
  console.log(`\n  Scenario result: ${passed}/${total} steps  ${status}`);
}

function printFinalSummary(allResults, scenarioResultsList, reportPath) {
  const totalPassed = allResults.filter((r) => r.passed).length;
  const totalFailed = allResults.length - totalPassed;

  console.log('\n' + THICK_LINE);
  const grpLabel = groupName ? `Group: ${groupName}  ·  ` : '';
  console.log(` CONFORMANCE SUMMARY  ·  ${grpLabel}${allResults.length} tests`);
  console.log(` Claimed App Version: ${groupVersion || 'unknown'}  ·  CTT Margo Version: ${cttMargoVersion}`);
  if (groupVersion && groupVersion !== cttMargoVersion) {
    console.log(
      `\x1b[32m⚠ Version Mismatch: Claimed App Version (${groupVersion}) differs from CTT Margo Version (${cttMargoVersion})\x1b[0m`
    );
  }
  console.log(THICK_LINE);

  // Per-scenario table
  const nameW = Math.max(28, ...scenarioResultsList.map((s) => s.name.length));
  const header = ` ${'Scenario'.padEnd(nameW)}  ${'Steps'.padStart(5)}  ${'Passed'.padStart(6)}  ${'Failed'.padStart(6)}`;
  console.log('');
  console.log(header);
  console.log(' ' + THIN_LINE.slice(0, header.length - 1));
  for (const s of scenarioResultsList) {
    const fail = s.total - s.passed;
    const failStr = fail > 0 ? String(fail) : ' 0';
    console.log(
      ` ${s.name.padEnd(nameW)}  ${String(s.total).padStart(5)}  ${String(s.passed).padStart(6)}  ${failStr.padStart(6)}`
    );
  }
  console.log(' ' + THIN_LINE.slice(0, header.length - 1));
  console.log(
    ` ${'TOTAL'.padEnd(nameW)}  ${String(allResults.length).padStart(5)}  ${String(totalPassed).padStart(6)}  ${String(totalFailed).padStart(6)}`
  );

  // List failed steps
  const failed = allResults.filter((r) => !r.passed);
  if (failed.length > 0) {
    console.log('\n FAILED TESTS:');
    for (const r of failed) {
      console.log(`\n  ✗  ${r.step}  [${r.scenario}]  ${r.name}`);
      console.log(`     ${r.method} ${r.endpoint}`);
      console.log(`     Expected ${httpLabel(r.expected)}  ·  Got ${httpLabel(r.actual)}`);
      const parts = r.reason.split('; ');
      for (const p of parts) { if (p.trim()) console.log(`     • ${p.trim()}`); }
    }
  }

  console.log('');
  const relReport = path.relative(process.cwd(), reportPath);
  console.log(` Report: ${relReport}`);
  console.log('');
  if (totalFailed === 0) {
    console.log(` ✅  ALL ${totalPassed} TESTS PASSED`);
  } else {
    console.log(` ❌  ${totalFailed} of ${allResults.length} TESTS FAILED`);
  }
  console.log(THICK_LINE + '\n');
}

// ─────────────────────────────────────────────────────────────────────────────
// Step execution
// ─────────────────────────────────────────────────────────────────────────────

// Sends a single HTTP request for a step (signing, Content-Digest, etc.) and
// returns the parsed response. Factored out of runStep so the polling loop
// below can re-issue the exact same request (with a fresh signature each
// time) on an interval.
async function performHTTPStep(step) {
  const method   = (step.method || 'GET').toUpperCase();
  const endpoint = substitute(step.endpoint || '');
  const url      = `${baseUrl}${endpoint}`;
  const headers  = { ...(substitute(step.headers || {})) };
  const body =
    step.request_body === undefined
      ? undefined
      : injectCertificate(substitute(step.request_body), step);
  const bodyText = body === undefined ? '' : JSON.stringify(body);

  if (bodyText) headers['Content-Type'] = headers['Content-Type'] || 'application/json';

  // step.mtls: true → MIAF transport (mutual TLS with an X.509-SVID) instead of
  // RFC 9421 request signing. The old flow's signature/digest headers don't exist
  // in MIAF at all, so this branch skips them entirely rather than layering on top.
  if (step.mtls) {
    if (!miafIdentity) {
      throw new Error(
        `step "${step.id}" has mtls:true but no SVID cert/key found at ${svidCertPath} / ${svidKeyPath} — ` +
        `see wfm-supplier/fixtures/miaf/README.md`
      );
    }
    const response = await request(method, url, headers, bodyText, miafIdentity);
    const parsed = parseBody(response.body);
    const responseSource = { ...parsed, _headers: response.headers, _body: response.body };
    return { method, endpoint, bodyText, response, responseSource };
  }

  // Prepare Content-Digest for body requests so the WFM doesn't reject due to missing
  // digest before it has a chance to check for the signature — unless the step is
  // specifically testing a missing/invalid Content-Digest, in which case it must be omitted.
  if (!step.skip_content_digest) prepareContentDigest(headers, bodyText);

  if (!step.skip_signing) signRequest(method, url, headers, bodyText);

  // "Signature verification failed" test: keep Signature-Input intact but flip one
  // character of the Signature value so the bytes no longer verify.
  if (step.tamper_signature && headers.Signature) {
    headers.Signature = headers.Signature.replace(/:([A-Za-z0-9+/=]+):/, (_m, b64) => {
      const chars = b64.split('');
      const i = Math.max(0, Math.floor(chars.length / 2));
      chars[i] = chars[i] === 'A' ? 'B' : 'A';
      return ':' + chars.join('') + ':';
    });
  }

  const response = await request(method, url, headers, bodyText);
  const parsed = parseBody(response.body);
  const responseSource = { ...parsed, _headers: response.headers, _body: response.body };
  return { method, endpoint, bodyText, response, responseSource };
}

// Application Registry checks: `oras` (or a plain HTTP GET for the raw tags-list
// endpoint) instead of an HTTP call to the WFM. Shaped exactly like
// performHTTPStep's return value so the rest of runStep (poll/validations/
// extract_context/report) needs no changes at all.
//   step.oras = { cmd: 'manifest' | 'tags' | 'blobs' | 'tags_raw', ref? }
// `ref` defaults to the REGISTRY_REF environment variable (falling back to our
// own spec-conformant fixture package) so the same test-case file works for
// any vendor's registry without editing JSON, but also runs out-of-the-box.
const DEFAULT_REGISTRY_REF = 'harbor.machine:8443/library/margo-ctt-hello-world:1.0.0';
async function performOrasStep(step) {
  const ref = step.oras.ref ? substitute(step.oras.ref) : process.env.REGISTRY_REF || DEFAULT_REGISTRY_REF;
  const repo = ref.replace(/(:[^/]*)?$/, ''); // strip a trailing ":tag", keep any port's colon (comes before the last "/")
  let status = 200;
  let body = '';
  let transportError;

  try {
    if (step.oras.cmd === 'manifest') {
      body = execFileSync('oras', ['manifest', 'fetch', ref], { encoding: 'utf8' });
    } else if (step.oras.cmd === 'tags') {
      const out = execFileSync('oras', ['repo', 'tags', ref], { encoding: 'utf8' });
      body = JSON.stringify({ tags: out.split('\n').map((s) => s.trim()).filter(Boolean) });
    } else if (step.oras.cmd === 'blobs') {
      const manifest = JSON.parse(execFileSync('oras', ['manifest', 'fetch', ref], { encoding: 'utf8' }));
      const failedDigests = [];
      for (const layer of manifest.layers || []) {
        try {
          execFileSync('oras', ['blob', 'fetch', '--output', '/dev/null', `${repo}@${layer.digest}`]);
        } catch {
          failedDigests.push(layer.digest);
        }
      }
      body = JSON.stringify({ layerCount: (manifest.layers || []).length, failedDigests });
    }
  } catch (err) {
    transportError = (err.stderr && err.stderr.toString().trim()) || err.message;
  }

  if (step.oras.cmd === 'tags_raw') {
    // Raw OCI Distribution "Listing Tags" endpoint — AR-010 checks the exact
    // {name, tags[]} JSON shape, which `oras repo tags` re-formats away.
    const [host, ...rest] = repo.split('/');
    const response = await request('GET', `https://${host}/v2/${rest.join('/')}/tags/list`, {}, '');
    const responseSource = { ...parseBody(response.body), _headers: response.headers, _body: response.body };
    return { method: 'GET', endpoint: ref, bodyText: '', response, responseSource };
  }

  const response = { status: transportError ? 0 : status, headers: {}, body, transportError };
  const responseSource = { ...parseBody(body), _headers: {}, _body: body };
  return { method: 'ORAS', endpoint: ref, bodyText: '', response, responseSource };
}

// Runs a list of validations against a response, returning an array of failure messages.
function runValidations(responseSource, validations) {
  const failures = [];
  for (const validation of validations || []) {
    const failure = validate(responseSource, validation);
    if (failure) failures.push(failure);
  }
  return failures;
}

async function runStep(scenario, step) {
  // fresh_cert: temporarily replace the global signing identity with a brand-new,
  // never-onboarded certificate so the WFM sees an unregistered signer.
  let savedCertState = null;
  if (step.fresh_cert) {
    const { execSync } = require('child_process');
    const tmpKey  = path.join(certDir, '.temp-fresh.key');
    const tmpCert = path.join(certDir, '.temp-fresh.pem');
    const tmpId   = `fresh-${Date.now()}`;
    try {
      execSync(`openssl ecparam -name prime256v1 -genkey -noout -out "${tmpKey}"`, { stdio: 'ignore' });
      execSync(
        `openssl req -new -x509 -days 1 -key "${tmpKey}" -out "${tmpCert}" -subj "/CN=${tmpId}"`,
        { stdio: 'ignore' }
      );
      savedCertState = { privateKey, deviceCertificate, deviceCertificateBase64, keyid };
      privateKey              = fs.readFileSync(tmpKey, 'utf8');
      deviceCertificate       = fs.readFileSync(tmpCert, 'utf8');
      deviceCertificateBase64 = Buffer.from(deviceCertificate).toString('base64');
      keyid                   = computeKeyId(privateKey);
    } catch (_) {
      // If temp cert generation fails, fall back to running without fresh-cert
    }
  }

  try {
    let method, endpoint, bodyText, response, responseSource;
    let pollTimedOut = false;
    let pollAttempts = 0;
    // A step with `oras: {...}` instead of `method`/`endpoint` checks the
    // Application Registry via the `oras` CLI (or a plain HTTP GET) rather than
    // the WFM's HTTP API — see performOrasStep(). Everything else (poll,
    // validations, extract_context, reporting) is unchanged.
    const performStep = step.oras ? performOrasStep : performHTTPStep;

    // step.poll = { interval_seconds, timeout_seconds, until: [validations] }
    // Re-issues this step's request on an interval until every validation in
    // `until` passes (e.g. "wait until the desired-state manifest lists >= 2
    // deployments", or "wait until it's back down to exactly 1") or the
    // timeout elapses — mirrors a real device-agent's state-seeking loop, and
    // lets a scenario wait on an operator making a change via the WFM's own
    // console mid-test (e.g. assigning or removing an app).
    if (step.poll) {
      const intervalSeconds = step.poll.interval_seconds ?? 5;
      const timeoutSeconds  = step.poll.timeout_seconds ?? 120;
      const deadline        = Date.now() + timeoutSeconds * 1000;
      const untilValidations = step.poll.until || [];

      for (;;) {
        pollAttempts++;
        ({ method, endpoint, bodyText, response, responseSource } = await performStep(step));

        const primaryMatchNow = response.status === step.expected_status;
        const pollFailures = primaryMatchNow
          ? runValidations(responseSource, untilValidations)
          : [`expected HTTP ${step.expected_status}, got ${response.status}`];

        if (pollFailures.length === 0) break;

        if (Date.now() >= deadline) {
          pollTimedOut = true;
          break;
        }

        console.log(
          `    ⏳ [poll attempt ${pollAttempts}] not ready yet (${pollFailures.join('; ')}) — retrying in ${intervalSeconds}s...`
        );
        await sleep(intervalSeconds * 1000);
      }
    } else {
      ({ method, endpoint, bodyText, response, responseSource } = await performStep(step));
    }

    const assertionFailures = [];

    // A step passes if the actual status matches expected OR any of the accepted alternatives.
    const acceptedStatuses = step.accepted_statuses || [];
    const primaryMatch     = response.status === step.expected_status;
    const alternativeMatch = !primaryMatch && acceptedStatuses.includes(response.status);
    const statusMatch      = primaryMatch || alternativeMatch;
    if (!statusMatch) {
      assertionFailures.push(`expected HTTP ${step.expected_status}, got ${response.status}`);
    }

    // Run field validations only when the PRIMARY expected status is matched.
    // When we got an accepted alternative (e.g. 404 instead of 200 for a bundle step),
    // the response body belongs to a different content type — validating it against the
    // success schema would produce false negatives, so we skip it.
    if (primaryMatch) {
      assertionFailures.push(...runValidations(responseSource, step.validations));
    }

    if (pollTimedOut) {
      assertionFailures.push(
        `timed out after ${step.poll.timeout_seconds ?? 120}s waiting for poll.until condition (${pollAttempts} attempt(s))`
      );
    }

    let newContext = {};
    if (assertionFailures.length === 0 && primaryMatch) {
      newContext = extractContext(responseSource, step.extract_context);
    }

    const passed = assertionFailures.length === 0 && !response.transportError;
    const resultEntry = {
      scenario: scenario.id,
      scenarioName: scenario.name,
      step: step.id,
      name: step.name,
      crIds: (step.crIds && step.crIds.length ? step.crIds : scenario.crIds) || [],
      method,
      endpoint,
      expected: step.expected_status,
      actual: response.status,
      passed,
      reason: failureSummary(response, assertionFailures),
    };
    results.push(resultEntry);

    printStepResult(step, resultEntry, newContext, bodyText);

    return passed;
  } finally {
    // Restore the original signing identity after fresh_cert steps
    if (savedCertState) {
      ({ privateKey, deviceCertificate, deviceCertificateBase64, keyid } = savedCertState);
      try { fs.unlinkSync(path.join(certDir, '.temp-fresh.key')); } catch (_) {}
      try { fs.unlinkSync(path.join(certDir, '.temp-fresh.pem')); } catch (_) {}
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// HTML report (unchanged logic, updated to use scenarioName)
// ─────────────────────────────────────────────────────────────────────────────

function htmlEscape(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function writeReport() {
  const passed = results.filter((r) => r.passed).length;
  const failed = results.length - passed;
  const grpLabel = groupName ? htmlEscape(groupName) : 'WFM Scenario';

  const rows = results
    .map(
      (r) => `
    <tr class="${r.passed ? 'pass' : 'fail'}">
      <td>${htmlEscape(r.passed ? 'PASS' : 'FAIL')}</td>
      <td>${htmlEscape(r.scenarioName || r.scenario)}</td>
      <td>${htmlEscape(r.step)}</td>
      <td>${htmlEscape(r.name)}</td>
      <td>${htmlEscape((r.crIds || []).join(', '))}</td>
      <td>${htmlEscape(r.method)}</td>
      <td>${htmlEscape(r.endpoint)}</td>
      <td>${htmlEscape(r.expected)}</td>
      <td>${htmlEscape(r.actual)}</td>
      <td>${htmlEscape(r.reason)}</td>
    </tr>`
    )
    .join('\n');

  // Per-scenario summary rows for HTML
  const scenarioRows = scenarioResults
    .map((s) => {
      const f = s.total - s.passed;
      return `
    <tr class="${f === 0 ? 'pass' : 'fail'}">
      <td>${htmlEscape(s.name)}</td>
      <td>${s.total}</td>
      <td>${s.passed}</td>
      <td>${f}</td>
    </tr>`;
    })
    .join('\n');

  const html = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>WFM Conformance Report — ${grpLabel}</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 24px; color: #1f2933; }
    h1 { font-size: 22px; margin-bottom: 4px; }
    h2 { font-size: 16px; margin: 24px 0 8px; }
    .meta { font-size: 13px; color: #555; margin-bottom: 18px; }
    .summary { margin-bottom: 10px; font-size: 15px; font-weight: bold; }
    .summary.all-pass { color: #166534; }
    .summary.has-fail { color: #b91c1c; }
    table { border-collapse: collapse; width: 100%; font-size: 13px; margin-bottom: 24px; }
    th, td { border: 1px solid #d7dde5; padding: 7px 9px; text-align: left; vertical-align: top; }
    th { background: #eef2f7; }
    tr.pass td:first-child { color: #166534; font-weight: 700; }
    tr.fail td:first-child, tr.fail td:last-child { color: #b91c1c; font-weight: 700; }
    .scenario-summary td:nth-child(4) { color: #b91c1c; }
    tr.pass.scenario-summary td:nth-child(4) { color: inherit; }
    .version-warning { margin-bottom: 14px; padding: 10px 14px; border-radius: 4px; background: #dcfce7; color: #166534; border: 1px solid #86efac; font-size: 13px; font-weight: bold; }
  </style>
</head>
<body>
  <h1>Margo WFM Conformance Report</h1>
  <div class="meta">
    Group: <strong>${grpLabel}</strong> &nbsp;|&nbsp;
    Claimed App Version: <strong>${htmlEscape(groupVersion || 'unknown')}</strong> &nbsp;|&nbsp;
    CTT Margo Version: <strong>${htmlEscape(cttMargoVersion)}</strong> &nbsp;|&nbsp;
    WFM: <strong>${htmlEscape(baseUrl)}</strong> &nbsp;|&nbsp;
    Run: <strong>${new Date().toISOString()}</strong>
  </div>
  ${
    groupVersion && groupVersion !== cttMargoVersion
      ? `<div class="version-warning">⚠ Version Mismatch: Claimed App Version (${htmlEscape(groupVersion)}) differs from CTT Margo Version (${htmlEscape(cttMargoVersion)})</div>`
      : ''
  }
  <div class="summary ${failed === 0 ? 'all-pass' : 'has-fail'}">
    ${failed === 0 ? '✅' : '❌'} ${passed} passed, ${failed} failed, ${results.length} total
  </div>

  ${(() => {
    const all = new Set();
    const covered = new Set();
    for (const r of results) {
      for (const c of r.crIds || []) {
        all.add(c);
        if (r.passed) covered.add(c);
      }
    }
    if (all.size === 0) return '';
    const list = [...all].sort().map((c) =>
      `<span style="display:inline-block;margin:2px 6px 2px 0;padding:2px 6px;border-radius:3px;font-size:12px;background:${covered.has(c) ? '#dcfce7' : '#fee2e2'};color:${covered.has(c) ? '#166534' : '#b91c1c'}">${htmlEscape(c)}</span>`
    ).join('');
    return `<div class="meta">Conformance requirements exercised: <strong>${covered.size} / ${all.size}</strong> passing</div><div style="margin-bottom:18px">${list}</div>`;
  })()}

  <h2>Scenario Summary</h2>
  <table>
    <thead>
      <tr><th>Scenario</th><th>Total</th><th>Passed</th><th>Failed</th></tr>
    </thead>
    <tbody class="scenario-summary">${scenarioRows}</tbody>
  </table>

  <h2>Step Details</h2>
  <table>
    <thead>
      <tr>
        <th>Status</th><th>Scenario</th><th>Step</th><th>Name</th><th>CR-IDs</th>
        <th>Method</th><th>Endpoint</th><th>Expected</th><th>Actual</th><th>Failure Reason</th>
      </tr>
    </thead>
    <tbody>${rows}</tbody>
  </table>
</body>
</html>`;

  fs.writeFileSync(reportFile, html);
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

(async () => {
  printBanner();

  const total = scenarios.length;

  for (let i = 0; i < scenarios.length; i++) {
    const scenario = scenarios[i];
    regenerateCertificate();
    context = {};

    printScenarioHeader(scenario, i + 1, total);

    let scenarioPassed = 0;
    for (const step of scenario.steps || []) {
      const ok = await runStep(scenario, step);
      if (ok) scenarioPassed++;
    }

    const stepCount = (scenario.steps || []).length;
    scenarioResults.push({ name: scenario.name, passed: scenarioPassed, total: stepCount });
    printScenarioSummary(scenarioPassed, stepCount);
  }

  writeReport();
  printFinalSummary(results, scenarioResults, reportFile);

  const failed = results.filter((r) => !r.passed).length;
  process.exit(failed > 0 ? 1 : 0);
})();
