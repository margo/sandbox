# Identity Lifecycle and Operator Playbooks

This document describes the identity lifecycle operations currently supported by the Margo Code First Sandbox. It maps the sandbox behavior to the [Margo Identity Lifecycle and Operator Playbooks](https://docs.margo.org/specification/identity/identity-lifecycle).

The procedures below are operator-driven. Automated enrollment, renewal, reissuance, and revocation protocols are not currently implemented by the sandbox.

## Lifecycle Support Summary

| Lifecycle operation | Sandbox implementation |
|---|---|
| Enrollment | MIS or an operator-provided channel generates an SVID certificate and private key. CSR-based issuance is not supported. |
| Renewal/reissuance | The operator generates a new certificate/key pair for the existing SPIFFE ID and replaces the device-agent identity files. The device-agent must be restarted. |
| Device-agent revocation | Remove the device-agent SPIFFE ID from the WFM authorization list. The agent watches the authorization file and applies the updated list to subsequent requests. |
| Trust bundle revocation | MIS supports a hard trust-bundle reset. Replace the Root CA and restart MIS. Clients refetch the updated bundle according to `spiffe_refresh_hint`. |
| Root CA hot reload | Intentionally not supported. New identities must be generated and distributed after a Root CA replacement. |

## Enrollment: Initial SVID Issuance

The sandbox currently uses centrally generated identity material:

1. Use the sandbox MIS or an operator-provided provisioning channel to generate an SVID certificate and its private key for the device-agent SPIFFE ID.
2. Copy the resulting certificate and key to the device-agent identity directory:

   ```text
   $HOME/sandbox/poc/agent/device/identity
   ```

3. Configure the device-agent to use the certificate and key, together with the required MIS HTTPS CA or trust-bundle configuration.
4. Start the device-agent and verify that it can establish the mTLS connection to the WFM.

The sandbox does **not** currently accept a CSR for SVID issuance. The operator or MIS therefore generates both the certificate and key.

For the supported SVID generation workflows, see the [binary getting started guide](./binary-getting-started.md).

## Renewal and Reissuance

To renew an SVID before it expires, or to reissue an SVID for the same device identity:

1. Generate a new SVID certificate and private key using the existing SPIFFE ID through MIS or an operator-provided channel.
2. Replace the device-agent identity files under:

   ```text
   $HOME/sandbox/poc/agent/device/identity
   ```

3. Restart the device-agent so it loads the new certificate and key.
4. Verify that the device-agent reconnects successfully to the WFM.

The SPIFFE ID must remain consistent with the authorization configuration. Replacing the certificate and key without restarting the service does not reload the identity material.

## Revoking Device-Agent Access

The sandbox does not provide certificate-level SVID revocation. To withdraw a device-agent's access to a WFM, remove the device-agent SPIFFE ID from the verifier's authorization list.

### Device-agent deployed by the setup scripts

Use option 7 in `wfm.sh` to remove the device-agent SPIFFE ID from the WFM configuration.

### Other deployments

Update the JSON authorization file that contains the device-agent SPIFFE ID and remove the entry. The device-agent watches this file, validates changes, and applies a valid updated list to subsequent requests. This is the most immediate revocation mechanism currently available in the sandbox.

This operation withdraws authorization at the application layer. It does not invalidate the SVID itself, and an already-established long-lived connection may need to be re-established before the change is observed.

## Trust Bundle Revocation and Root CA Replacement

Trust-bundle revocation is a broader operation than removing one device-agent from an authorization list. The current MIS implementation supports a hard reset of the trust bundle:

1. Obtain or issue the replacement Root CA through the existing PKI/operator channels.
2. Replace the Root CA currently supplied by MIS with the updated Root CA.
3. Restart MIS. A service interruption is expected because MIS does not hot-reload the Root CA.
4. Generate and distribute new identities that chain to the replacement Root CA.
5. Confirm that clients refresh and use the updated trust bundle.

The expected MIS downtime should remain within the configured `spiffe_refresh_hint` interval, which clients use to determine when to refetch trust-bundle data. During a compromise response, this operation intentionally blocks identities that still chain to the replaced trust anchor once the updated bundle has propagated.

Root CA hot reload is intentionally not supported. Continuing to use existing identities after a Root CA replacement would not provide the intended compromise response; new identities must be generated and supplied to the affected principals.

## Operational Limitations

- CSR-based SVID issuance is not supported.
- Certificate renewal and reissuance are manual and require a device-agent restart.
- Device-agent revocation is authorization-list removal, not cryptographic certificate revocation.
- Trust-bundle reset requires an MIS restart and replacement identities.
- Revocation is not necessarily instantaneous for already-established mTLS sessions; session behavior depends on connection re-establishment and verifier policy evaluation.
- Keep-alive connections between the device-agent and WFM are disabled. Each request creates a new connection, ensuring the WFM authorization allowlist is checked for every call.
