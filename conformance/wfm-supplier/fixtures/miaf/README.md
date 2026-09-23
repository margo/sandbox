# MIAF (mTLS) test identity — placeholder fixtures

These are **throwaway, self-signed** SPIFFE-shaped certificates, generated
locally with `openssl` (no real MIS involved), so the runner's new
`mtls: true` step path (see `run_wfm_scenarios.js`) has something to load and
can be exercised/verified without a real Margo Identity Service reachable.

They are **not** conformance-meaningful on their own — a real conformance run
against a real MIAF-speaking WFM needs a properly minted X.509-SVID from that
WFM's actual trust domain (see `CONFORMANCE_FLOWS_AND_MIAF_MIGRATION.md` §5.7
and §8 open question 8 — where our real SVID gets minted from is still an
open decision).

## Files

- `ca.pem` / `ca-key.pem` — a throwaway test CA (stands in for a trust domain's MIS).
- `wfm-svid-cert.pem` / `wfm-svid-key.pem` — a test "WFM" identity
  (`spiffe://margo-ctt.test/margo/wfm/margo-ctt-test-wfm`), useful if we ever
  need to *play* the WFM side (e.g. a future device-supplier mock).
- `client-svid-cert.pem` / `client-svid-key.pem` — a test "WFM Client / device"
  identity (`spiffe://margo-ctt.test/margo/wfm/margo-ctt-test-wfm/client/margo-ctt-test-device`).

## How to use with `run_wfm_scenarios.js`

Copy (or symlink) the client identity into whatever `<cert-dir>` you pass on
the command line, using these exact names:

```
cp client-svid-cert.pem <cert-dir>/svid-cert.pem
cp client-svid-key.pem  <cert-dir>/svid-key.pem
cp ca.pem               <cert-dir>/svid-ca.pem
```

The runner loads `svid-cert.pem` / `svid-key.pem` / `svid-ca.pem` from the
cert dir **only if present** — everything else (the existing RFC 9421
identity files, every scenario that doesn't set `mtls: true`) is completely
unaffected. Any step with `"mtls": true` in its scenario JSON will then
present this SVID instead of RFC-9421-signing the request.

## Swapping in a real, operator-minted SVID later

Once a real MIS is reachable and someone mints us a proper SVID, just
overwrite `svid-cert.pem` / `svid-key.pem` / `svid-ca.pem` in the cert dir
(or replace these fixture files and re-copy) — **no code change required**.

## Regenerating these test fixtures

```bash
openssl ecparam -name prime256v1 -genkey -noout -out ca-key.pem
openssl req -new -x509 -key ca-key.pem -days 3650 -out ca.pem \
  -subj "/CN=margo-ctt-test-ca" -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,keyCertSign,cRLSign"

# then for each identity (wfm-svid / client-svid):
openssl ecparam -name prime256v1 -genkey -noout -out <name>-key.pem
openssl req -new -key <name>-key.pem -subj "/CN=<name>" -out <name>.csr
openssl x509 -req -in <name>.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial \
  -out <name>-cert.pem -days 365 \
  -extfile <(printf "basicConstraints=critical,CA:false\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth,clientAuth\nsubjectAltName=URI:<spiffe-id>")
```
