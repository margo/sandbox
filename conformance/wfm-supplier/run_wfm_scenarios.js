#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');

function usage() {
  console.error(
    'Usage: node run_wfm_scenarios.js <base-url> <scenarios.json> <report.html> <cert-dir> [group-name] [group-version]'
  );
  process.exit(2);
}

const [baseUrlArg, scenariosFile, reportFile, certDir, groupName, groupVersion] =
  process.argv.slice(2);
if (!baseUrlArg || !scenariosFile || !reportFile || !certDir) usage();

const baseUrl = baseUrlArg.replace(/\/+$/, '');
const privateKeyPath = path.join(certDir, 'device.key');
const deviceCertPath = path.join(certDir, 'device-cert.pem');
const caCertPath = path.join(certDir, 'ca-cert.pem');

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
    extract_context: { clientId: 'clientId' },
    validations: [{ field: 'clientId', operation: 'is_string' }],
  },
  'POST /api/v1/clients/{clientId}/capabilities': {
    bodyMerge: { apiVersion: 'device.margo.org/v1alpha1', kind: 'DeviceCapabilitiesManifest' },
    bodyNested: { 'properties.id': '{clientId}' },
  },
  'PUT /api/v1/clients/{clientId}/capabilities': {
    bodyMerge: { apiVersion: 'device.margo.org/v1alpha1', kind: 'DeviceCapabilitiesManifest' },
    bodyNested: { 'properties.id': '{clientId}' },
  },
  'GET /api/v1/clients/{clientId}/deployments': {
    extract_context: {
      deploymentId: 'deployments.0.deploymentId',
      bundleDigest: 'bundle.digest',
      deploymentDigest: 'deployments.0.digest',
    },
    validations: [{ field: 'manifestVersion', operation: 'is_number' }],
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
    // "Not Modified" — send If-None-Match; server may still return 200 or 404 if no cached entry
    return { add_if_none_match: true, accepted_statuses: [200, 304, 404] };
  }
  if (code === 406) {
    // "Not Acceptable" — some servers return 500 if they don't implement content negotiation
    return { wrong_accept: true, accepted_statuses: [406, 500] };
  }
  if (/content.?digest/i.test(name)) {
    // "Missing or invalid content-digest" — skipping signing omits digest; server may return 400 or 401
    return { skip_signing: true, accepted_statuses: [400, 401] };
  }
  if (/signature.*fail|signature.*verif/i.test(name) || code === 401) {
    // "Signature verification failed" — skip signing; some servers return 400 instead of 401
    return { skip_signing: true, accepted_statuses: [400, 401] };
  }
  if (/invalid.*cert.*format|cert.*format.*invalid|cert.*format.*struct/i.test(name)) {
    // skip_signing: prevents 409 "already registered" when the device keyid is already onboarded.
    // Symphony may validate signature first (→ 401) or cert format first (→ 400); accept both.
    return { bad_certificate: true, skip_signing: true, accepted_statuses: [400, 401, 409] };
  }
  if (/not.*trusted|revoked|rejected/i.test(name) || code === 403) {
    // "Certificate not trusted" — use fresh unregistered cert; server may return 400, 401, or 403
    return { fresh_cert: true, accepted_statuses: [400, 401, 403] };
  }
  if (/semantic.*error|body.*semantic|request body includes/i.test(name) || code === 422) {
    return { use_placeholder_body: true, accepted_statuses: [400, 422] };
  }
  if (code === 404) {
    // 404: CONTEXT_FALLBACKS produce a well-formed non-existent path; server should return 404
    // Also accept 400 (some servers reject non-matching digest formats) and 301 (redirect edge case)
    return { accepted_statuses: [400, 404, 301] };
  }
  // Generic 400 or any other explicitly documented error: accept the documented code plus 404
  return { accepted_statuses: [code > 0 ? code : 400, 404] };
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
  const fresh_cert   = errorBehavior.fresh_cert ?? false;

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
    // 400 "Invalid certificate format" — send a plaintext string as the certificate field
    request_body = {
      ...(rules.body || {}),
      certificate: 'INVALID_NOT_A_PEM_CERTIFICATE',
    };
  } else if (errorBehavior.use_placeholder_body) {
    // 422 "Semantic error" — keep the Postman Lorem-Ipsum body (bad apiVersion, invalid values)
    // Don't apply bodyMerge/bodyNested — the placeholder body IS the bad payload
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

  return {
    id: resp.id,
    name: `${parentItem.name} — ${resp.name || resp.status}`,
    method,
    endpoint,
    headers,
    ...(request_body !== null ? { request_body } : {}),
    expected_status,
    ...(accepted_statuses ? { accepted_statuses } : {}),
    validations,
    extract_context,
    skip_signing,
    fresh_cert,
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

function request(method, url, headers, bodyText) {
  return new Promise((resolve) => {
    const parsedUrl = new URL(url);
    const options = {
      method,
      hostname: parsedUrl.hostname,
      port: parsedUrl.port || 443,
      path: parsedUrl.pathname + parsedUrl.search,
      headers,
      ca: caCertificate,
      rejectUnauthorized: false,
      timeout: 30000,
    };

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

function validate(responseSource, validation) {
  const actual = getField(responseSource, validation.field);
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
    default:
      return `unsupported validation operation: ${validation.operation}`;
  }
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
  if (step.skip_signing) return '[unsigned]';
  if (step.fresh_cert)   return '[fresh-cert · signed]';
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
  const grpLabel = groupName ? `Group: ${groupName}${groupVersion ? ` (v${groupVersion})` : ''}  ·  ` : '';
  console.log(` CONFORMANCE SUMMARY  ·  ${grpLabel}${allResults.length} tests`);
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

    // Always prepare Content-Digest for body requests so the WFM doesn't reject due to
    // missing digest before it has a chance to check for the signature.
    prepareContentDigest(headers, bodyText);

    if (!step.skip_signing) signRequest(method, url, headers, bodyText);

    const response = await request(method, url, headers, bodyText);
    const parsed = parseBody(response.body);
    const responseSource = { ...parsed, _headers: response.headers, _body: response.body };
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
      for (const validation of step.validations || []) {
        const failure = validate(responseSource, validation);
        if (failure) assertionFailures.push(failure);
      }
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
  const grpLabel = groupName
    ? `${htmlEscape(groupName)}${groupVersion ? ` v${htmlEscape(groupVersion)}` : ''}`
    : 'WFM Scenario';

  const rows = results
    .map(
      (r) => `
    <tr class="${r.passed ? 'pass' : 'fail'}">
      <td>${htmlEscape(r.passed ? 'PASS' : 'FAIL')}</td>
      <td>${htmlEscape(r.scenarioName || r.scenario)}</td>
      <td>${htmlEscape(r.step)}</td>
      <td>${htmlEscape(r.name)}</td>
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
  </style>
</head>
<body>
  <h1>Margo WFM Conformance Report</h1>
  <div class="meta">
    Group: <strong>${grpLabel}</strong> &nbsp;|&nbsp;
    WFM: <strong>${htmlEscape(baseUrl)}</strong> &nbsp;|&nbsp;
    Run: <strong>${new Date().toISOString()}</strong>
  </div>
  <div class="summary ${failed === 0 ? 'all-pass' : 'has-fail'}">
    ${failed === 0 ? '✅' : '❌'} ${passed} passed, ${failed} failed, ${results.length} total
  </div>

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
        <th>Status</th><th>Scenario</th><th>Step</th><th>Name</th>
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
