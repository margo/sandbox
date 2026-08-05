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

#------------------------------------------------------------------------------
# Adds or updates NodeHosts entries in the CoreDNS ConfigMap.
#------------------------------------------------------------------------------

add_or_update_host() {
        local ip="$1"
        local host="$2"

        if awk -v host="$host" '
            /^[[:space:]]*#/ { next }
            $2 == host { found=1 }
            END { exit !found }
        ' <<<"$updated"; then

            updated=$(
                awk -v ip="$ip" -v host="$host" '
                    /^[[:space:]]*#/ {
                        print
                        next
                    }

                    $2 == host {
                        print ip " " host
                        next
                    }

                    {
                        print
                    }
                ' <<<"$updated"
            )

        else
            if [[ -n "$updated" ]]; then
                updated+=$'\n'
            fi
            updated+="${ip} ${host}"
        fi
}

configure_coredns_hosts() {
    set -euo pipefail

    local namespace="kube-system"
    local configmap="coredns"

    local harbor_host="${EXPOSED_HARBOR_HOST}"
    local symphony_host="${WFM_HOST}"

    local harbor_ip
    local symphony_ip

    local nodehosts
    local updated

    echo "[INFO] Validating dependencies..."

    for cmd in kubectl jq awk sed; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            echo "[ERROR] Required command '$cmd' is not installed." >&2
            return 1
        fi
    done

    echo "[INFO] Reading IP addresses from /etc/hosts..."

    harbor_ip=$(get_ip_from_hosts "$harbor_host")
    symphony_ip=$(get_ip_from_hosts "$symphony_host")

    echo "[INFO] Verifying CoreDNS ConfigMap..."

    if ! kubectl -n "$namespace" get configmap "$configmap" >/dev/null 2>&1; then
        echo "[ERROR] ConfigMap '${configmap}' not found in namespace '${namespace}'." >&2
        return 1
    fi

    nodehosts=$(
        kubectl -n "$namespace" get configmap "$configmap" \
            -o jsonpath='{.data.NodeHosts}' 2>/dev/null || true
    )

    updated="$nodehosts"

    add_or_update_host "$harbor_ip" "$harbor_host"
    add_or_update_host "$symphony_ip" "$symphony_host"

    updated=$(sed '/^[[:space:]]*$/d' <<<"$updated")

    if [[ "$updated" == "$nodehosts" ]]; then
        echo "[INFO] CoreDNS NodeHosts already up to date."
        return 0
    fi

    echo "[INFO] Updating CoreDNS ConfigMap..."

    kubectl -n "$namespace" patch configmap "$configmap" \
        --type merge \
        --patch "$(cat <<EOF
{
  "data": {
    "NodeHosts": $(printf '%s' "$updated" | jq -Rs .)
  }
}
EOF
)"

    echo "[INFO] Restarting CoreDNS..."

    kubectl -n "$namespace" rollout restart deployment/coredns

    echo "[INFO] Waiting for CoreDNS rollout to complete..."

    kubectl -n "$namespace" rollout status deployment/coredns --timeout=180s

    echo "[INFO] CoreDNS NodeHosts updated successfully."
}