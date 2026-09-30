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

## Sandbox Operator Playbooks (Script-Based Setup)

The following procedures apply when the sandbox has been deployed using the setup scripts (`mis.sh`, `wfm.sh`, `device-agent.sh`) as described in the [Setting Up the Code First Sandbox](./setup-guide.md) guide. These steps provide reliable, script-assisted paths for each lifecycle operation.

---

### Enrollment

Enrollment is covered as part of the initial setup. Refer to [Step 3: Build Everything — Generate X.509-SVIDs](./setup-guide.md#generate-x509-svids-for-wfm-and-wfm-client) in the setup guide. No additional enrollment steps are required after initial setup unless a new principal is being added.

---

### Renewal and Reissuance

Renewal or reissuance replaces the certificate and key for an existing SPIFFE ID. The SPIFFE ID must remain consistent with the authorization configuration unless you intentionally intend to change it (see note below).

> **Note on SPIFFE ID change:** If the SPIFFE ID changes during reissuance, the new SPIFFE ID must be added to the relevant authorization allowlist before the renewed identity will be accepted. See [Device-Agent Revocation](#device-agent-revocation-sandbox) and [WFM Revocation on Device Agent](#wfm-revocation-on-device-agent-sandbox) for allowlist management steps.

#### Step 1 — Generate a New Certificate and Key Pair

On the **WFM VM**, navigate to the scripts folder and run the SVID generator:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash mis.sh
```

- A menu will appear.
- Type `6` and press Enter.
- Choose: `Option 6: Generate SVID`

Follow the interactive prompts to select the principal (WFM or WFM Client) and provide the SPIFFE ID. The generator produces:

```
payload-cert.pem   # X.509 SVID certificate
payload-key.pem    # Corresponding private key
```

These files are placed in a subdirectory under `$HOME/workspace/sandbox/scripts/` named after the principal ID you provided (e.g., `x509svid-wfm`, `x509svid-wfm-docker-client`, `x509svid-wfm-helm-client`).

#### Step 2 — Place the New Files in the Correct Identity Directory

After generating the new certificate and key, copy them to the appropriate identity directory for the target principal.

---

##### Renewing/Reissuing the WFM Identity

Copy the new files to the WFM identity directory:

```bash
cp $HOME/workspace/sandbox/scripts/x509svid-<wfm-id>/payload-cert.pem $HOME/symphony/api/certificates/payload-cert.pem
cp $HOME/workspace/sandbox/scripts/x509svid-<wfm-id>/payload-key.pem $HOME/symphony/api/certificates/payload-key.pem
```

Replace `<wfm-id>` with the WFM principal ID used during SVID generation (e.g., `wfm`).

Then restart the WFM service so it loads the new identity material:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash wfm.sh
```

- Type `4` and press Enter — `Option 4: Symphony Stop` (Do not wipe off redis data)
- Re-run and type `3` and press Enter — `Option 3: Symphony Start`

Verify the WFM reconnects successfully by checking its logs:

```bash
sudo docker logs -f symphony-api-container
```
> Note: If you change the `<wfm-id>` while SVID Generation, you need to regenerate WFM Client SVIDs as well and place them appropriately, as WFM Client identities are paired to a particular WFM. Previous identities are rendered useless in that case. 

---

##### Renewing/Reissuing the Compose-Capable Device Agent Identity

On the **WFM VM**, generate the new SVID as described in Step 1 (select the WFM Client principal for the compose-capable device).

Transfer the new files to the **Compose-Capable Device VM**:

```bash
# Run from the Compose-Capable Device VM
scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-<compose-client-id>/payload-cert.pem \
    $HOME/sandbox/docker-compose/config/identity/payload-cert.pem

scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-<compose-client-id>/payload-key.pem \
    $HOME/sandbox/docker-compose/config/identity/payload-key.pem
```

Replace `<compose-client-id>` with the WFM client ID used during SVID generation (e.g., `wfm-docker-client`).

Then restart the device agent on the **Compose-Capable Device VM**:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash device-agent.sh docker
```

- Type `4` and press Enter — `Option 4: Device-agent-Stop`
- Re-run and type `3` and press Enter — `Option 3: Device-agent-Start(docker-compose-device)`

Verify the device agent reconnects successfully:

```bash
sudo docker logs -f workload-fleet-management-client
```

---

##### Renewing/Reissuing the Helm-Capable Device Agent Identity

On the **WFM VM**, generate the new SVID as described in Step 1 (select the WFM Client principal for the helm-capable device).

Transfer the new files to the **Helm-Capable Device VM**:

```bash
# Run from the Helm-Capable Device VM
scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-<helm-client-id>/payload-cert.pem \
    $HOME/sandbox/helmchart/config/helm-identity/payload-cert.pem

scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-<helm-client-id>/payload-key.pem \
    $HOME/sandbox/helmchart/config/helm-identity/payload-key.pem
```

Replace `<helm-client-id>` with the WFM client ID used during SVID generation (e.g., `wfm-helm-client`).

Then restart the device agent on the **Helm-Capable Device VM**:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash device-agent.sh k3s
```

- Type `6` and press Enter — `Option 6: Device-agent-Stop` (do not wipe off data)
- Re-run and type `5` and press Enter — `Option 5: Device-agent-Start(k3s-device)`

Verify the device agent reconnects successfully (replace `<pod-name>` with the actual pod name):

```bash
sudo kubectl logs -f <pod-name> -n default
```

---

### Device-Agent Revocation (Sandbox)

To revoke a device agent's access to the WFM, remove its SPIFFE ID from the WFM authorization allowlist.

On the **WFM VM**:

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash wfm.sh
```

- A menu will appear.
- Type `7` and press Enter.
- Choose: `Option 7: Manage SPIFFE ID allowlist`

Follow the interactive prompts to **remove** the SPIFFE ID of the WFM client (device agent) whose access should be revoked.

The WFM watches the authorization file and applies the updated list to subsequent requests. Already-established connections may need to be re-established before the change takes effect. Keep-alive connections are disabled in the sandbox, so each new request will be evaluated against the updated allowlist.

---

### WFM Revocation on Device Agent (Sandbox)

To revoke the WFM's authorization on the device agent side (i.e., prevent the device agent from accepting requests from the current WFM SPIFFE ID), update the device agent's local authorization policy.

> **Important:** The device agent's authorization policy must not be left empty. An empty authorization policy means no communication with any WFM is permitted. Use the edit option to replace the existing WFM SPIFFE ID with the correct or updated one rather than removing it outright.

**On the Compose-Capable Device VM:**

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash device-agent.sh docker
```

- A menu will appear.
- Type `11` and press Enter.
- Choose: `Option 11: Manage SPIFFE ID allowlist`

Follow the interactive prompts to **edit** the WFM SPIFFE ID entry, replacing it with the updated or replacement SPIFFE ID. This effectively revokes the previous WFM's access on the device agent side.

**On the Helm-Capable Device VM:**

```bash
cd $HOME/workspace/sandbox/scripts
sudo -E bash device-agent.sh k3s
```

- A menu will appear.
- Type `11` and press Enter.
- Choose: `Option 11: Manage SPIFFE ID allowlist`

Follow the interactive prompts to **edit** the WFM SPIFFE ID entry as described above.

---

### Trust Bundle Revocation and Root CA Replacement (Sandbox)

To perform a hard trust bundle reset in the sandbox:

#### Step 1 — Replace the Root CA Files

On the **WFM VM**, replace the existing SVID Root CA certificate and key with the new ones, keeping the filenames unchanged:

```bash
cp <path-to-new-ca.crt> $HOME/mis-deployment/certs/ca.crt
cp <path-to-new-ca.key> $HOME/mis-deployment/certs/ca.key
```

> **Do not rename the files.** MIS expects `ca.crt` and `ca.key` at this path. If you want to rename the files, then MIS configuration must be changed to support it.

#### Step 2 — Restart the MIS Container

Restart MIS to load the new Root CA:

```bash
sudo docker restart margo-identity-service
```

A brief service interruption is expected. MIS does not hot-reload the Root CA. Clients will refetch the updated trust bundle according to the configured `spiffe_refresh_hint` interval.

#### Step 3 — Generate and Distribute New Identities

After MIS restarts with the new Root CA, all existing SVIDs chain to the old (now replaced) trust anchor and will no longer be trusted. Generate new SVIDs for all principals and distribute them using the `[Renewal and Reissuance](#renewal-and-reissuance)` steps above:

1. Generate new SVIDs for WFM and all WFM clients (compose and helm capable devices) using `mis.sh` Option 6.
2. Place the new `payload-cert.pem` and `payload-key.pem` in the correct identity directories for each principal.
3. Restart each service (WFM and device agents) to load the new identity material.
4. Verify that each principal reconnects successfully and that the updated trust bundle has propagated.

---

## Operational Limitations

- CSR-based SVID issuance is not supported.
- Certificate renewal and reissuance are manual and require a device-agent restart.
- Device-agent revocation is authorization-list removal, not cryptographic certificate revocation.
- Trust-bundle reset requires an MIS restart and replacement identities.
- Revocation is not necessarily instantaneous for already-established mTLS sessions; session behavior depends on connection re-establishment and verifier policy evaluation.
- Keep-alive connections between the device-agent and WFM are disabled. Each request creates a new connection, ensuring the WFM authorization allowlist is checked for every call.
