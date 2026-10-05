#!/bin/bash

#------------------------------------------------------------------------------
# Returns the IPv4 address of the specified hostname from /etc/hosts.
#------------------------------------------------------------------------------
get_ip_from_hosts() {
    local hostname="$1"
    local hosts_file="/etc/hosts"
    local ip

    if [[ ! -r "$hosts_file" ]]; then
        echo "[ERROR] Unable to read ${hosts_file}." >&2
        return 1
    fi

    ip=$(
        awk -v host="$hostname" '
            /^[[:space:]]*#/ { next }
            NF >= 2 && $2 == host {
                print $1
                exit
            }
        ' "$hosts_file"
    )

    if [[ -z "$ip" ]]; then
        echo "[ERROR] Host '${hostname}' not found in ${hosts_file}." >&2
        return 1
    fi


    if [[ ! "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
        echo "[ERROR] Invalid IPv4 address '${ip}' for '${hostname}'." >&2
        return 1
    fi

    printf '%s\n' "$ip"
}

configure_coredns_hosts() {
    set -euo pipefail

    local namespace="kube-system"
    local yaml_file="custom-coredns-hosts.yaml"

    # Use defaults if env variables are not exported
    local harbor_host="${EXPOSED_HARBOR_HOST:-harbor.machine}"
    local symphony_host="${WFM_HOST:-symphony.machine}"
    local mis_host="${EXPOSED_MIS_HOST:-mis.margo.org}"

    local harbor_ip
    local symphony_ip
    local mis_host_ip

    echo "[INFO] Validating dependencies..."

    for cmd in kubectl; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            echo "[ERROR] Required command '$cmd' is not installed." >&2
            return 1
        fi
    done

    echo "[INFO] Reading IP addresses from /etc/hosts for ($harbor_host, $symphony_host, $mis_host)..."

    harbor_ip=$(get_ip_from_hosts "$harbor_host") || return 1
    symphony_ip=$(get_ip_from_hosts "$symphony_host") || return 1
    mis_host_ip=$(get_ip_from_hosts "$mis_host") || return 1

    echo "[INFO] Resolved IPs — $harbor_host: $harbor_ip | $symphony_host: $symphony_ip | $mis_host: $mis_host_ip"

    echo "[INFO] Generating $yaml_file..."

    cat > "$yaml_file" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: ${namespace}
data:
  custom.server: |
    ${symphony_host}:53 ${harbor_host}:53 ${mis_host}:53 {
        hosts {
          ${harbor_ip} ${harbor_host}
          ${symphony_ip} ${symphony_host}
          ${mis_host_ip} ${mis_host}
          fallthrough
        }
        cache 30
        log
        errors
    }
EOF

    echo "[INFO] $yaml_file created successfully."

    echo "[INFO] Applying $yaml_file to cluster..."

    if ! kubectl apply -f "$yaml_file"; then
        echo "[ERROR] Failed to apply $yaml_file." >&2
        return 1
    fi

    echo "[INFO] ConfigMap 'coredns-custom' applied successfully."

    echo "[INFO] Restarting CoreDNS deployment..."

    if ! kubectl -n "$namespace" rollout restart deployment/coredns; then
        echo "[ERROR] Failed to restart CoreDNS deployment." >&2
        return 1
    fi

    echo "[INFO] Waiting for CoreDNS rollout to complete (timeout: 60s)..."

    if ! kubectl -n "$namespace" rollout status deployment/coredns --timeout=60s; then
        echo "[ERROR] CoreDNS rollout did not complete within 60 seconds." >&2
        return 1
    fi

    echo "[INFO] CoreDNS restarted and running successfully."
    echo "[INFO] Custom host entries are now active for: $harbor_host, $symphony_host, $mis_host"
}