# Real MIAF identity + trust bundle (2026-09-23)

Unlike `../` (the throwaway self-signed placeholder), everything in this
folder is **real**, obtained from the actual MIS deployed for this
environment (`mis.margo.org:9443`, trust domain `margo.org`):

- `client-svid-cert.pem` / `client-svid-key.pem` — a real X.509-SVID minted by
  that MIS via `scripts/lib/mis/svid-gen.sh --automated --principal wfm-client
  --spiffe-id spiffe://margo.org/margo/wfm/symphony-1/client/margo-ctt-device`
  (90-day TTL, minted 2026-09-23). This is our conformance suite's identity
  when acting as a WFM Client under Symphony's WFM (`symphony-1`).
- `trust-bundle-ca.pem` — the trust domain's real root CA, extracted from
  `GET https://mis.margo.org:9443/.well-known/spiffe/bundle.json` via
  `node run_wfm_scenarios.js --fetch-trust-bundle <mis-url> <output.pem>`
  (see that command's implementation for the one-time-fetch flow).

Verified: `openssl verify -CAfile trust-bundle-ca.pem client-svid-cert.pem`
→ `OK` — the minted SVID genuinely chains to the published trust bundle.

## Using these with `run_wfm_scenarios.js`

Copy into a cert dir under the same fixed names the runner looks for:

```
cp client-svid-cert.pem <cert-dir>/svid-cert.pem
cp client-svid-key.pem  <cert-dir>/svid-key.pem
cp trust-bundle-ca.pem  <cert-dir>/svid-ca.pem
```

## Known environment limitation (not a code issue)

`--fetch-trust-bundle` needs to be pointed at `https://127.0.0.1:9443` (not
`https://mis.margo.org:9443`) when run **from this same VM**: the discovery
document correctly returns the absolute `https://mis.margo.org:9443/...`
trustBundleUri per spec, but this VM's public IP doesn't hairpin back to
itself on port 9443 (works fine on 8082/8443, likely a security-group rule
scoped to just those ports). From any *other* machine, the real hostname
works normally. Worth asking the team to open 9443 if this ever needs to run
from off-box.

## Regenerating

Re-run the mint command above (new SVID, up to a new 90-day TTL) and/or
re-run `--fetch-trust-bundle` — both are safe to redo any time; nothing here
is order-dependent on the other.
