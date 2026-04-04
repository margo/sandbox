# Mock WFM Postman Assets

This folder exposes the `mock-wfm` comprehensive conformance coverage as a Postman collection so team members can run, duplicate, and extend the cases without editing shell scripts.

## Files

- `mock-wfm-comprehensive.postman_collection.json`
  - Main collection.
  - Mirrors the comprehensive shell suite by category.
  - Saves dynamic values such as deployment digests, bundle digests, and ETags into collection variables.
- `mock-wfm-local.postman_environment.json`
  - Ready-to-import local environment.
  - Defaults to `http://localhost:8090/v1alpha2/margo`.

## Import

1. Open Postman.
2. Import `mock-wfm-comprehensive.postman_collection.json`.
3. Import `mock-wfm-local.postman_environment.json`.
4. Select the `Mock WFM Local` environment.

## Run

1. Start the mock server:

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
./mock-wfm
```

2. In Postman, run the collection with the Collection Runner.
3. Keep the request order as-is.

Order matters because the collection captures:

- `deploymentId`
- `deploymentDigest`
- `bundleDigest`
- `manifestETag`
- `bundleETag`

from the deployments manifest before the dependent deployment and bundle requests run.

## HTTPS

To run against HTTPS instead of local HTTP, update the environment `baseUrl`, for example:

```text
https://10.139.9.90:9090/v1alpha2/margo
```

If Postman is verifying TLS, trust the corresponding `ca-cert.pem`.

## Extending

Recommended pattern for new tests:

1. Duplicate the closest existing request.
2. Change only the request body, headers, or URL needed for the new scenario.
3. Keep assertions small and explicit in the request `Tests` tab.
4. If a new request depends on data from an earlier response, save it with `pm.collectionVariables.set(...)`.

## Notes

- The collection is designed to be generic with respect to client IDs and deployment references.
- Negative cases focus on OpenAPI-driven validation boundaries and content negotiation behavior.
- Some requests assert multiple conditions inside one Postman request, so Postman request count will not exactly match shell test count.
