#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
    cat <<'EOF'
Usage:
  ./run.sh portman [BASE_URL]
  ./run.sh newman [COLLECTION_FILE]
  ./run.sh execute
  ./run.sh cleanup
  ./run.sh all [BASE_URL]

Commands:
  portman   Generate collection and test data from OpenAPI via Portman.
  newman    Run API validations on a collection (defaults to postman_collection.json).
  execute   Execute deployed workloads and verify with docker ps.
  cleanup   Stop and remove deployed containers.
  all       Full workflow: portman → newman → execute (requires WFM running).

Examples:
  ./run.sh all https://symphony.machine:8082/v1alpha2/margo
  ./run.sh all                    (uses default URL)
  ./run.sh newman                 (rerun API tests only)
  ./run.sh execute                (deploy and verify workloads)
  ./run.sh cleanup                (clean up containers)
EOF
}

cmd="${1:-}"
arg1="${2:-}"

if [[ -z "$cmd" ]]; then
    usage
    exit 2
fi

case "$cmd" in
    portman)
        exec "$SCRIPT_DIR/1-setup_portman.sh" "$arg1"
        ;;
    newman)
        exec "$SCRIPT_DIR/2-run_newman.sh" "$arg1"
        ;;
    execute)
        exec "$SCRIPT_DIR/3-execute-workloads.sh"
        ;;
    cleanup)
        exec "$SCRIPT_DIR/4-cleanup.sh"
        ;;
    all)
        echo "Starting full conformance workflow..."
        echo ""
        "$SCRIPT_DIR/1-setup_portman.sh" "$arg1" || { echo "Setup failed"; exit 1; }
        echo ""
        "$SCRIPT_DIR/2-run_newman.sh" || { echo "API tests failed"; exit 1; }
        echo ""
        "$SCRIPT_DIR/3-execute-workloads.sh" || { echo "Workload execution failed"; exit 1; }
        echo ""
        echo "✅ Full conformance workflow complete!"
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        echo "Unknown command: $cmd"
        usage
        exit 2
        ;;
esac
