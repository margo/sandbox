#!/usr/bin/env bash
# =============================================================================
# mis-manager.sh — Margo Identity Service (MIS) Manager
# =============================================================================

set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────
RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'
BOLD=$'\033[1m'
RESET=$'\033[0m'

# ── Helpers ───────────────────────────────────────────────────────────────────
info()    { echo -e "${CYAN}[INFO]${RESET}  $*"; }
success() { echo -e "${GREEN}[OK]${RESET}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
error()   { echo -e "${RED}[ERROR]${RESET} $*" >&2; }
die()     { error "$*"; exit 1; }

banner() {
    echo -e "${BOLD}"
    echo "╔══════════════════════════════════════════════════════════╗"
    echo "║          Margo Identity Service (MIS) Manager           ║"
    echo "╚══════════════════════════════════════════════════════════╝"
    echo -e "${RESET}"
}

# ── SPIFFE trust-domain validation ───────────────────────────────────────────
# Per SPIFFE spec: trust domain = one or more dot-separated labels,
# each label: [a-z0-9]([a-z0-9-]*[a-z0-9])?  (lowercase, no leading/trailing dash)
validate_trust_domain() {
    local td="$1"
    # Must not be empty, must not start/end with dot, each label valid
    if [[ -z "$td" ]]; then
        return 1
    fi
    # Reject if starts or ends with dot
    if [[ "$td" == .* || "$td" == *. ]]; then
        return 1
    fi
    # Split by dot and validate each label
    IFS='.' read -ra labels <<< "$td"
    for label in "${labels[@]}"; do
        if [[ -z "$label" ]]; then
            return 1
        fi
        # Must match [a-z0-9]([a-z0-9-]*[a-z0-9])?
        if ! [[ "$label" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ || "$label" =~ ^[a-z0-9]$ ]]; then
            return 1
        fi
    done
    return 0
}

# ── Shell RC file detection ───────────────────────────────────────────────────
# Returns a list of rc files that exist for the current user
get_shell_rc_files() {
    local rc_files=()
    local home="${HOME}"

    # Always target these common files if they exist (or create them)
    local candidates=(
        "${home}/.bashrc"
        "${home}/.bash_profile"
        "${home}/.zshrc"
        "${home}/.profile"
        "${home}/.kshrc"
        "${home}/.config/fish/config.fish"
    )

    for f in "${candidates[@]}"; do
        if [[ -f "$f" ]]; then
            rc_files+=("$f")
        fi
    done

    # Always include .profile as a POSIX fallback (create if missing)
    if [[ ! -f "${home}/.profile" ]]; then
        touch "${home}/.profile"
        rc_files+=("${home}/.profile")
    fi

    # Deduplicate
    printf '%s\n' "${rc_files[@]}" | sort -u
}

# ── SVID function block markers ───────────────────────────────────────────────
SVID_MARKER_START="# >>> MIS svid-gen BEGIN <<<"
SVID_MARKER_END="# >>> MIS svid-gen END <<<"

svid_function_block() {
    cat <<'EOF'
svid-gen() {
    sudo -E bash "$HOME/workspace/sandbox/scripts/mis.sh" generate-svid
}
EOF
}

# ── Inject svid-gen into all detected shell rc files ─────────────────────────
inject_svid_function() {
    info "Injecting svid-gen function into shell configuration files..."

    local block
    block=$(printf '%s\n%s\n%s\n' \
        "$SVID_MARKER_START" \
        "$(svid_function_block)" \
        "$SVID_MARKER_END")

    local injected=0
    while IFS= read -r rc_file; do
        [[ -z "$rc_file" ]] && continue

        # Skip fish — different syntax
        if [[ "$rc_file" == *.fish ]]; then
            warn "Skipping fish shell ($rc_file) — manual setup required."
            warn "  Add to $rc_file:"
            warn "    function svid-gen"
            warn "      sudo -E bash \/workspace/sandbox/scripts/mis.sh generate-svid"
            warn "    end"
            continue
        fi

        # Remove any existing block first (idempotent)
        if grep -q "$SVID_MARKER_START" "$rc_file" 2>/dev/null; then
            # Use a temp file for portability (no sed -i on all platforms)
            local tmp
            tmp=$(mktemp)
            awk "/$SVID_MARKER_START/{found=1} !found{print} /$SVID_MARKER_END/{found=0}" \
                "$rc_file" > "$tmp"
            mv "$tmp" "$rc_file"
        fi

        # Append the block
        printf '\n%s\n' "$block" >> "$rc_file"
        success "  → Injected into $rc_file"
        injected=$((injected + 1))
    done < <(get_shell_rc_files)

    if [[ $injected -eq 0 ]]; then
        warn "No shell rc files found. Creating ~/.profile with svid-gen."
        printf '\n%s\n' "$block" >> "${HOME}/.profile"
    fi
}

# ── Remove svid-gen from all shell rc files ───────────────────────────────────
remove_svid_function() {
    info "Removing svid-gen function from shell configuration files..."

    while IFS= read -r rc_file; do
        [[ -z "$rc_file" ]] && continue
        if grep -q "$SVID_MARKER_START" "$rc_file" 2>/dev/null; then
            local tmp
            tmp=$(mktemp)
            awk "/$SVID_MARKER_START/{found=1} !found{print} /$SVID_MARKER_END/{found=0}" \
                "$rc_file" > "$tmp"
            mv "$tmp" "$rc_file"
            success "  → Removed from $rc_file"
        fi
    done < <(get_shell_rc_files)
}

# ── Activate svid-gen in the CURRENT shell session ───────────────────────────
activate_svid_now() {
    # Export the function definition so it's available immediately
    # in the current shell (works for bash; zsh users get it on next login)
    svid-gen() {
        sudo -E bash "$HOME/workspace/sandbox/scripts/mis.sh" generate-svid
    }
    export -f svid-gen 2>/dev/null || true
    success "svid-gen is now available in this shell session."
}

# ── Deactivate svid-gen in the CURRENT shell session ─────────────────────────
deactivate_svid_now() {
    unset -f svid-gen 2>/dev/null || true
    success "svid-gen unloaded from current shell session."
}

# ── /etc/hosts management ─────────────────────────────────────────────────────
 HOSTS_MARKER="# MIS managed entry"

add_hosts_entry() {
    local ip="$1"
    local hostname="$2"
    local entry="${ip}    ${hostname}    ${HOSTS_MARKER}"

    # Remove any existing MIS-managed entry for this hostname
    sudo sed -i "/ ${hostname} .*${HOSTS_MARKER}/d" /etc/hosts
    sudo sed -i "/ ${hostname}$/d" /etc/hosts

    echo "$entry" | sudo tee -a /etc/hosts > /dev/null
    success "Added to /etc/hosts: ${entry}"
}

remove_hosts_entry() {
    if grep -q "${HOSTS_MARKER}" /etc/hosts 2>/dev/null; then
        sudo sed -i "/${HOSTS_MARKER}/d" /etc/hosts
        success "Removed MIS entry from /etc/hosts."
    fi
}

# =============================================================================
# FLAG: --help
# =============================================================================
cmd_help() {
    banner
    cat <<EOF
${BOLD}USAGE${RESET}
    $(basename "$0") <FLAG>

${BOLD}FLAGS${RESET}

  ${GREEN}--install${RESET}
      Installs Margo Identity Service (MIS) in full capacity.

      You will be prompted for:
        • Trust Domain  (SPIFFE-conformant, e.g. margo.org)
        • Image tag     (Sandbox e.g. v1.0.0-rc.3)

      Steps performed:
        1. Install system dependencies (git)
        2. Clone the Margo sandbox repository (sparse checkout)
        3. Patch mis.env with your trust domain and tag
        4. Run MIS installation stages:
             mis.sh install
             mis.sh setup-factory   → PKI certs written to ~/mis-deployment/certs/
             mis.sh mis-install     → MIS server fully operational
        5. Run basic diagnostics (container status + TLS curl)
        6. Register the ${BOLD}svid-gen${RESET} shell function globally
           (available immediately in this session and all future shells)

      After install, add the following to your host machine's /etc/hosts:
        <THIS_VM_IP>  mis.<TRUST_DOMAIN>

  ${GREEN}--uninstall${RESET}
      Completely removes MIS from this machine.

      Steps performed:
        1. mis.sh mis-uninstall
        2. mis.sh uninstall
        3. Removes ~/workspace directory
        4. Removes the svid-gen function from all shell rc files

      Docker itself is NOT removed.

  ${GREEN}--restart-mis${RESET}
      Restarts the margo-identity-service Docker container.
        docker restart margo-identity-service
    ${GREEN}--status${RESET}
        Shows the current MIS installation state.

        Reports on:
        • Workspace & scripts presence
        • mis.env configuration
        • PKI certificate availability
        • Docker container existence and running state
        • svid-gen shell registration

        Outputs one of three states:
        ✔ Fully installed  — shows available actions including svid-gen usage
        ⚠ Partially installed — recommends cleanup and reinstall steps
        ✘ Not installed    — shows prerequisites and install steps

  ${GREEN}--help${RESET}
      Displays this help message.

${BOLD}EXAMPLES${RESET}
    $(basename "$0") --install
    $(basename "$0") --restart-mis
    $(basename "$0") --status
    $(basename "$0") --uninstall

${BOLD}NOTES${RESET}
    • Requires sudo privileges for MIS operations.
    • Docker must already be installed before running --install.
    • svid-gen generates SVIDs for WFM / WFM Client principals.
EOF
}



# =============================================================================
# FLAG: --status
# =============================================================================
cmd_status() {
    banner
    info "Checking MIS installation status..."
    echo ""

    local container_exists=false
    local container_running=false
    local workspace_exists=false
    local mis_env_exists=false
    local certs_exist=false
    local svid_gen_available=false

    # ── Check container ───────────────────────────────────────────────────────
    if docker inspect margo-identity-service > /dev/null 2>&1; then
        container_exists=true
        if docker ps --filter "name=margo-identity-service" \
                     --filter "status=running" \
                     --format "{{.Names}}" | grep -q "margo-identity-service"; then
            container_running=true
        fi
    fi

    # ── Check workspace / scripts ─────────────────────────────────────────────
    [[ -d "$HOME/workspace/sandbox/scripts" ]] && workspace_exists=true

    # ── Check mis.env ─────────────────────────────────────────────────────────
    [[ -f "$HOME/workspace/sandbox/scripts/mis.env" ]] && mis_env_exists=true

    # ── Check certs ───────────────────────────────────────────────────────────
    [[ -d "$HOME/mis-deployment/certs" && \
       -f "$HOME/mis-deployment/certs/https-ca.crt" ]] && certs_exist=true

    # ── Check svid-gen ────────────────────────────────────────────────────────
    if declare -f svid-gen > /dev/null 2>&1 || \
       grep -q "MIS svid-gen BEGIN" "${HOME}/.bashrc" \
                                    "${HOME}/.bash_profile" \
                                    "${HOME}/.zshrc" \
                                    "${HOME}/.profile" 2>/dev/null; then
        svid_gen_available=true
    fi

    # ── Print component table ─────────────────────────────────────────────────
    echo -e "${BOLD}Component Status:${RESET}"
    echo ""

    _status_row() {
        local label="$1"
        local ok="$2"
        if [[ "$ok" == "true" ]]; then
            echo -e "  ${GREEN}✔${RESET}  $label"
        else
            echo -e "  ${RED}✘${RESET}  $label"
        fi
    }

    _status_row "Workspace & scripts  ($HOME/workspace/sandbox/scripts)" "$workspace_exists"
    _status_row "mis.env configured   ($HOME/workspace/sandbox/scripts/mis.env)" "$mis_env_exists"
    _status_row "PKI certs present    ($HOME/mis-deployment/certs/https-ca.crt)" "$certs_exist"
    _status_row "Container exists     (margo-identity-service)" "$container_exists"
    _status_row "Container running    (margo-identity-service)" "$container_running"
    _status_row "svid-gen registered  (shell rc / current session)" "$svid_gen_available"
    echo ""

    # ── Determine overall state ───────────────────────────────────────────────

    # FULLY INSTALLED & RUNNING
    if $container_running && $workspace_exists && $mis_env_exists && \
       $certs_exist && $svid_gen_available; then

        echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════════╗${RESET}"
        echo -e "${BOLD}${GREEN}║        MIS is fully installed and operational            ║${RESET}"
        echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════════╝${RESET}"
        echo ""

        # Pull trust domain from mis.env for display
        local trust_domain=""
        if $mis_env_exists; then
            trust_domain=$(grep "^EXPOSED_MIS_HOST=" \
                "$HOME/workspace/sandbox/scripts/mis.env" 2>/dev/null \
                | sed 's/^EXPOSED_MIS_HOST=mis\.//')
        fi

        local mis_port=""
        if $mis_env_exists; then
            mis_port=$(grep "^EXPOSED_MIS_PORT=" \
                "$HOME/workspace/sandbox/scripts/mis.env" 2>/dev/null \
                | sed 's/^EXPOSED_MIS_PORT=//')
        fi

        local this_ip
        this_ip=$(hostname -I | awk '{print $1}')

        [[ -n "$trust_domain" ]] && \
            info "Trust Domain : ${trust_domain}" && \
            info "MIS Endpoint : https://mis.${trust_domain}"
        info "Certs Dir    : ${HOME}/mis-deployment/certs/"
        info "VM IP        : ${this_ip}"
        echo ""

        echo -e "${BOLD}Available actions:${RESET}"
        echo ""
        echo -e "  ${CYAN}svid-gen${RESET}"
        echo -e "    Generate SVIDs and private keys for WFM / WFM Client principals."
        echo -e "    Run this command in your terminal (available in current session"
        echo -e "    and all future shells)."
        echo ""
        echo -e "  ${CYAN}$(basename "$0") --restart-mis${RESET}"
        echo -e "    Restart the MIS container if it becomes unresponsive."
        echo ""
        echo -e "  ${CYAN}$(basename "$0") --uninstall${RESET}"
        echo -e "    Completely remove MIS from this machine."
        echo ""

        # Live health check
        info "Testing HTTPS connectivity to MIS..."
        local ca_cert="${HOME}/mis-deployment/certs/https-ca.crt"
        if [[ -n "$trust_domain" && -f "$ca_cert" ]]; then
            if curl --silent --fail --max-time 10 \
                    --cacert "$ca_cert" \
                    "https://mis.${trust_domain}:${mis_port}/.well-known/margo" \
                    > /dev/null 2>&1; then
                success "MIS Discovery endpoint is reachable."
            else
                warn "MIS Discovery Endpoint did not respond — container may still be warming up."
                warn "Retry: curl --cacert ${ca_cert} https://mis.${trust_domain}:${mis_port}/.well-known/margo"
            fi
        else
            warn "Skipping health check (trust domain or CA cert not resolved)."
        fi

    # PARTIALLY INSTALLED — various degraded states
    elif $workspace_exists || $mis_env_exists || $certs_exist || $container_exists; then

        echo -e "${BOLD}${YELLOW}╔══════════════════════════════════════════════════════════╗${RESET}"
        echo -e "${BOLD}${YELLOW}║          MIS is PARTIALLY installed (degraded)           ║${RESET}"
        echo -e "${BOLD}${YELLOW}╚══════════════════════════════════════════════════════════╝${RESET}"
        echo ""
        warn "Some components are present but MIS is not fully operational."
        echo ""

        echo -e "${BOLD}Likely causes:${RESET}"
        ! $container_exists  && echo -e "  ${RED}✘${RESET}  Container missing — mis.sh mis-install may not have completed."
        $container_exists && ! $container_running && \
            echo -e "  ${RED}✘${RESET}  Container exists but is stopped — it may have crashed."
        ! $certs_exist       && echo -e "  ${RED}✘${RESET}  PKI certs missing — mis.sh setup-factory may not have run."
        ! $workspace_exists  && echo -e "  ${RED}✘${RESET}  Workspace missing — repository was not cloned or was deleted."
        ! $svid_gen_available && echo -e "  ${RED}✘${RESET}  svid-gen not registered in shell rc files."
        echo ""

        echo -e "${BOLD}Recommended steps:${RESET}"
        echo ""
        echo -e "  1. Clean up the partial installation:"
        echo -e "     ${CYAN}$(basename "$0") --uninstall${RESET}"
        echo ""
        echo -e "  2. Re-run a fresh installation:"
        echo -e "     ${CYAN}$(basename "$0") --install${RESET}"
        echo ""
        warn "Do NOT skip --uninstall — leftover state can cause mis-install to fail."

    # NOT INSTALLED
    else

        echo -e "${BOLD}${RED}╔══════════════════════════════════════════════════════════╗${RESET}"
        echo -e "${BOLD}${RED}║              MIS is NOT installed                        ║${RESET}"
        echo -e "${BOLD}${RED}╚══════════════════════════════════════════════════════════╝${RESET}"
        echo ""
        info "No MIS components were detected on this machine."
        echo ""

        echo -e "${BOLD}Prerequisites before installing:${RESET}"
        echo -e "  • Docker must be installed and running."
        echo -e "  • sudo privileges are required."
        echo -e "  • Internet access to reach github.com/margo/sandbox."
        echo ""

        echo -e "${BOLD}Installation steps:${RESET}"
        echo ""
        echo -e "  1. Run the installer:"
        echo -e "     ${CYAN}$(basename "$0") --install${RESET}"
        echo ""
        echo -e "  2. When prompted, provide:"
        echo -e "     • ${BOLD}Trust Domain${RESET}  — SPIFFE-conformant, e.g. ${CYAN}margo.org${RESET}"
        echo -e "     • ${BOLD}Image tag${RESET}     — Sandbox release, e.g. ${CYAN}v1.0.0-rc.3${RESET}"
        echo ""
        echo -e "  3. After install, add to ${BOLD}/etc/hosts${RESET} on each host machine (WFM / WFM Client):"
        echo -e "     ${CYAN}<THIS_VM_IP>    mis.<TRUST_DOMAIN>${RESET}"
        echo ""
        echo -e "  4. Use ${CYAN}svid-gen${RESET} to generate SVIDs for your principals."
        echo ""
        info "For full details: $(basename "$0") --help"

    fi

    echo ""
}

# =============================================================================
# FLAG: --install
# =============================================================================
cmd_install() {
     banner
    
    # ── Guard: prevent double installation ───────────────────────────────────
    if docker inspect margo-identity-service > /dev/null 2>&1; then
        if docker ps --filter "name=margo-identity-service" \
                     --filter "status=running" \
                     --format "{{.Names}}" | grep -q "margo-identity-service"; then
            echo ""
            error "MIS is already installed and running."
            echo ""
            echo -e "  Container ${BOLD}margo-identity-service${RESET} is currently active."
            echo ""
            echo -e "  ${YELLOW}Options:${RESET}"
            echo -e "    • To restart MIS   : $(basename "$0") --restart-mis"
            echo -e "    • To reinstall MIS : $(basename "$0") --uninstall  then  $(basename "$0") --install"
            echo ""
            exit 1
        else
            # Container exists but is stopped — warn but allow reinstall
            warn "Container 'margo-identity-service' exists but is not running."
            warn "It may be from a previous failed or stopped installation."
            read -rp "$(echo -e "${YELLOW}Proceed with fresh install anyway? [y/N]: ${RESET}")" proceed
            if [[ ! "$proceed" =~ ^[Yy]$ ]]; then
                info "Install cancelled. Run --uninstall first to clean up, then retry."
                exit 0
            fi
        fi
    fi

    info "Starting MIS installation wizard..."
    echo ""

    # ── 1. Prompt: Trust Domain ───────────────────────────────────────────────
    local default_td="margo.org"
    local trust_domain=""

    while true; do
        read -rp "$(echo -e "${BOLD}Trust Domain${RESET} [default: ${default_td}]: ")" trust_domain
        trust_domain="${trust_domain:-$default_td}"

        if validate_trust_domain "$trust_domain"; then
            success "Trust domain accepted: ${trust_domain}"
            break
        else
            error "'${trust_domain}' is not a valid SPIFFE trust domain."
            echo "  Rules: lowercase labels separated by dots, e.g. margo.org"
            echo "  Each label: [a-z0-9] with optional internal hyphens."
        fi
    done

    # ── 2. Prompt: Tag ────────────────────────────────────────────────────────
    local default_tag="v1.0.0-rc.3"
    local tag=""

    read -rp "$(echo -e "${BOLD}MIS release tag${RESET} [default: ${default_tag}]: ")" tag
    tag="${tag:-$default_tag}"
    info "Using tag: ${tag}"
    echo ""

    # ── 2b. Prompt: MIS Port ──────────────────────────────────────────────────
    local default_port="9443"
    local mis_port=""
    
    read -rp "$(echo -e "${BOLD}MIS Port${RESET} [default: ${default_port}]: ")" mis_port
    mis_port="${mis_port:-$default_port}"
    info "Using MIS port: ${mis_port}"
    echo ""

    # ── 3. Working directory ──────────────────────────────────────────────────
    cd "$HOME"
    info "Working directory: $HOME"

    # ── 4. Install git ────────────────────────────────────────────────────────
    info "Updating package lists and installing git..."
    sudo apt-get update -y
    sudo apt-get install git -y
    success "git installed."

    # ── 5. Clone sandbox (sparse) ─────────────────────────────────────────────
    info "Setting up workspace..."
    mkdir -p workspace
    cd workspace

    if [[ -d "sandbox/.git" ]]; then
        warn "sandbox directory already exists — skipping clone."
    else
        git clone --filter=blob:none --sparse \
            https://github.com/margo/sandbox.git
    fi

    cd sandbox
    git sparse-checkout init --no-cone
    git sparse-checkout set scripts/

    # ── 6. Checkout tag ───────────────────────────────────────────────────────
    info "Checking out tag: ${tag}..."
    if ! git fetch --tags --quiet 2>&1; then
        warn "Could not fetch tags — proceeding with local refs."
    fi

    if ! git checkout "$tag" 2>&1; then
        die "Tag '${tag}' does not exist in the repository or checkout failed. Aborting."
    fi
    success "Checked out tag: ${tag}"

    # ── 7. Patch mis.env ──────────────────────────────────────────────────────
    local mis_env="$HOME/workspace/sandbox/scripts/mis.env"

    if [[ ! -f "$mis_env" ]]; then
        die "mis.env not found at ${mis_env}. Aborting."
    fi

    info "Patching mis.env..."

    # EXPOSED_MIS_HOST=mis.<trust_domain>
    if grep -q "^export EXPOSED_MIS_HOST=" "$mis_env"; then
        sed -i "s|^export EXPOSED_MIS_HOST=.*|export EXPOSED_MIS_HOST=mis.${trust_domain}|" "$mis_env"
    else
        echo "export EXPOSED_MIS_HOST=mis.${trust_domain}" >> "$mis_env"
    fi

    # SANDBOX_REPO_BRANCH=<tag>
    if grep -q "^export SANDBOX_REPO_BRANCH=" "$mis_env"; then
        sed -i "s|^export SANDBOX_REPO_BRANCH=.*|export SANDBOX_REPO_BRANCH=${tag}|" "$mis_env"
    else
        echo "export SANDBOX_REPO_BRANCH=${tag}" >> "$mis_env"
    fi

    # EXPOSED_MIS_PORT=<port>
    if grep -q "^export EXPOSED_MIS_PORT=" "$mis_env"; then
        sed -i "s|^export EXPOSED_MIS_PORT=.*|export EXPOSED_MIS_PORT=${mis_port}|" "$mis_env"
    else
        echo "export EXPOSED_MIS_PORT=${mis_port}" >> "$mis_env"
    fi

    success "mis.env updated:"
    success "  EXPOSED_MIS_HOST  = mis.${trust_domain}"
    success "  SANDBOX_REPO_BRANCH = ${tag}"
    success "  EXPOSED_MIS_PORT  = ${mis_port}"

    # ── 7b. Patch mis_IMAGE_TAG in mis.sh ────────────────────────────────────
    local mis_sh="$HOME/workspace/sandbox/scripts/mis.sh"

    if [[ ! -f "$mis_sh" ]]; then
        die "mis.sh not found at ${mis_sh}. Aborting."
    fi

    info "Patching mis_IMAGE_TAG in mis.sh..."
    if grep -q "^mis_IMAGE_TAG=" "$mis_sh"; then
        sed -i "s|^mis_IMAGE_TAG=.*|mis_IMAGE_TAG=${tag}|" "$mis_sh"
    else
        warn "mis_IMAGE_TAG not found in mis.sh — appending it."
        echo "mis_IMAGE_TAG=${tag}" >> "$mis_sh"
    fi
    success "mis.sh updated: mis_IMAGE_TAG = ${tag}"

    # ── 8. mis.sh install ─────────────────────────────────────────────────────
    cd "$HOME/workspace/sandbox/scripts"

    info "Running: mis.sh install ..."
    if ! sudo -E bash mis.sh install; then
        die "'mis.sh install' failed. Aborting."
    fi
    success "mis.sh install completed."

    # ── 9. mis.sh setup-factory ───────────────────────────────────────────────
    info "Running: mis.sh setup-factory ..."
    if ! sudo -E bash mis.sh setup-factory; then
        die "'mis.sh setup-factory' failed. Aborting."
    fi
    success "mis.sh setup-factory completed."

    echo ""
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}${CYAN}║              PKI Certificates Location                  ║${RESET}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════════╝${RESET}"
    echo -e "  All PKI certificates are located at:"
    echo -e "  ${BOLD}${HOME}/mis-deployment/certs/${RESET}"
    echo ""

    # ── 10. mis.sh mis-install ────────────────────────────────────────────────
    info "Running: mis.sh mis-install ..."
    if ! sudo -E bash mis.sh mis-install; then
        die "'mis.sh mis-install' failed. Aborting."
    fi
    success "MIS installation complete."

# ── 11. /etc/hosts — add entry on this machine ────────────────────────────
    local this_ip
    this_ip=$(hostname -I | awk '{print $1}')
    local mis_hostname="mis.${trust_domain}"

    echo ""
    info "Adding ${mis_hostname} → ${this_ip} to /etc/hosts..."
    add_hosts_entry "$this_ip" "$mis_hostname"

    echo ""
    echo -e "${BOLD}${YELLOW}╔══════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}${YELLOW}║           /etc/hosts Entry Required on Host VMs          ║${RESET}"
    echo -e "${BOLD}${YELLOW}╚══════════════════════════════════════════════════════════╝${RESET}"
    echo -e "  Also add the following line to ${BOLD}/etc/hosts${RESET} on each"
    echo -e "  ${BOLD}other${RESET} host machine (WFM, WFM Client) that needs to reach MIS:"
    echo ""
    echo -e "  ${BOLD}${this_ip}    ${mis_hostname}${RESET}"
    echo ""
    echo -e "  MIS server is accessible at: ${BOLD}https://${mis_hostname}${RESET}"
    echo ""

    # ── 12. Diagnostics ───────────────────────────────────────────────────────
    info "Running diagnostics..."

    echo ""
    info "Checking margo-identity-service container status..."
    if docker ps --filter "name=margo-identity-service" \
                 --filter "status=running" \
                 --format "{{.Names}}" | grep -q "margo-identity-service"; then
        success "Container 'margo-identity-service' is running."
    else
        warn "Container 'margo-identity-service' does not appear to be running."
        warn "Check with: docker ps -a | grep margo-identity-service"
    fi

    echo ""
    info "Testing HTTPS connectivity to MIS (using CA cert)..."
    local ca_cert="${HOME}/mis-deployment/certs/https-ca.crt"
    if [[ -f "$ca_cert" ]]; then
       if curl --silent --fail --max-time 10 \
               --cacert "$ca_cert" \
               "https://mis.${trust_domain}:${mis_port}/.well-known/margo" \
               > /dev/null 2>&1; then
           success "MIS HTTPS endpoint is reachable."
       else
           warn "MIS HTTPS endpoint did not respond (may still be starting up)."
           warn "Retry: curl --cacert ${ca_cert} https://mis.${trust_domain}:${mis_port}/.well-known/margo"
       fi
    else
        warn "CA cert not found at ${ca_cert} — skipping curl check."
    fi

    # ── 13. Inject svid-gen ───────────────────────────────────────────────────
    echo ""
    inject_svid_function
    activate_svid_now

    # ── 13. Inject svid-gen ───────────────────────────────────────────────────
    echo ""
    inject_svid_function
    activate_svid_now
    
    # ── Source hint ───────────────────────────────────────────────────────────
    echo ""
    echo -e "${BOLD}${YELLOW}ACTION REQUIRED — Activate svid-gen in your current shell:${RESET}"
    echo ""
    
    # Detect which rc file was written to and suggest sourcing it
    local shell_rc=""
    if [[ -f "${HOME}/.bashrc" ]]; then
        shell_rc="${HOME}/.bashrc"
    elif [[ -f "${HOME}/.zshrc" ]]; then
        shell_rc="${HOME}/.zshrc"
    elif [[ -f "${HOME}/.profile" ]]; then
        shell_rc="${HOME}/.profile"
    fi
    
    if [[ -n "$shell_rc" ]]; then
        echo -e "  Run this command in your terminal:"
        echo ""
        echo -e "  ${CYAN}source ${shell_rc}${RESET}"
        echo ""
        echo -e "  Or simply run:"
        echo -e "  ${CYAN}svid-gen() { sudo -E bash \"\/workspace/sandbox/scripts/mis.sh\" generate-svid; }${RESET}"
    else
        echo -e "  ${CYAN}svid-gen() { sudo -E bash \"\/workspace/sandbox/scripts/mis.sh\" generate-svid; }${RESET}"
    fi
    echo ""# ── 13. Inject svid-gen ───────────────────────────────────────────────────
    echo ""
    inject_svid_function
    activate_svid_now

    # ── 14. Final Summary ─────────────────────────────────────────────────────
    echo ""
    echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}${GREEN}║                 Installation Summary                    ║${RESET}"
    echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════════╝${RESET}"
    echo ""
    echo -e "  ${BOLD}What to take to your host machines (WFM / WFM Client):${RESET}"
    echo ""
    echo -e "  1. ${BOLD}MIS Normative API CA Certificate${RESET}"
    echo -e "     Path : ${HOME}/mis-deployment/certs/https-ca.crt"
    echo -e "     Copy this certificate to your WFM / WFM Client machines"
    echo -e "     and configure them to trust it."
    echo ""
    echo -e "  2. ${BOLD}Generated SVIDs & Keys${RESET}"
    echo -e "     Use the ${BOLD}svid-gen${RESET} command (available after sourcing or relogin)"
    echo -e "     to generate SVIDs and private keys for your principals:"
    echo ""
    echo -e "     ${CYAN}svid-gen${RESET}"
    echo ""
    echo -e "     Follow the prompts to generate SVIDs for:"
    echo -e "       • WFM (Workflow Manager)"
    echo -e "       • WFM Client"
    echo ""
    echo -e "  3. ${BOLD}/etc/hosts entry${RESET} on each host machine:"
    echo -e "     ${this_ip}    mis.${trust_domain}"
    echo ""
    echo -e "  ${BOLD}Trust Domain  :${RESET} ${trust_domain}"
    echo -e "  ${BOLD}MIS Port      :${RESET} ${mis_port}"
    echo -e "  ${BOLD}MIS Endpoint  :${RESET} https://mis.${trust_domain}"
    echo -e "  ${BOLD}Certs Dir     :${RESET} ${HOME}/mis-deployment/certs/"


     # ── Source hint ───────────────────────────────────────────────────────────
    echo ""
    echo -e "${BOLD}${YELLOW}ACTION REQUIRED — Activate svid-gen in your current shell:${RESET}"
    echo ""
    
    # Detect which rc file was written to and suggest sourcing it
    local shell_rc=""
    if [[ -f "${HOME}/.bashrc" ]]; then
        shell_rc="${HOME}/.bashrc"
    elif [[ -f "${HOME}/.zshrc" ]]; then
        shell_rc="${HOME}/.zshrc"
    elif [[ -f "${HOME}/.profile" ]]; then
        shell_rc="${HOME}/.profile"
    fi
    
    if [[ -n "$shell_rc" ]]; then
        echo -e "  Run this command in your terminal for ${BOLD}svid-gen${RESET} command:"
        echo ""
        echo -e "  ${CYAN}source ${shell_rc}${RESET}"
        echo ""
    fi
    echo ""

    success "MIS is fully installed and operational."
}

# =============================================================================
# FLAG: --uninstall
# =============================================================================
cmd_uninstall() {
    banner
    warn "This will completely remove MIS from this machine."
    read -rp "$(echo -e "${YELLOW}Are you sure? [y/N]: ${RESET}")" confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        info "Uninstall cancelled."
        exit 0
    fi

    local scripts_dir="$HOME/workspace/sandbox/scripts"

    if [[ ! -d "$scripts_dir" ]]; then
        warn "Scripts directory not found at ${scripts_dir}."
        warn "MIS may not be installed, or workspace was already removed."
    else
        cd "$scripts_dir"

        # ── 1. mis-uninstall ──────────────────────────────────────────────────
        info "Running: mis.sh mis-uninstall ..."
        if sudo -E bash mis.sh mis-uninstall; then
            success "mis.sh mis-uninstall completed."
        else
            warn "mis.sh mis-uninstall encountered errors (continuing)."
        fi

        # ── 2. uninstall ──────────────────────────────────────────────────────
        info "Running: mis.sh uninstall ..."
        if sudo -E bash mis.sh uninstall; then
            success "mis.sh uninstall completed."
        else
            warn "mis.sh uninstall encountered errors (continuing)."
        fi
    fi

    # ── 3. Remove workspace ───────────────────────────────────────────────────
    info "Removing ${HOME}/workspace ..."
    rm -rf "${HOME}/workspace"
    success "Workspace removed."

    # ── 4. Remove svid-gen from shells ───────────────────────────────────────
    remove_svid_function
    deactivate_svid_now
    remove_hosts_entry

    echo ""
    success "MIS has been fully uninstalled."
    info "Docker and system packages were NOT removed."
}

# =============================================================================
# FLAG: --restart-mis
# =============================================================================
cmd_restart_mis() {
    banner
    info "Restarting margo-identity-service container..."

    if ! docker inspect margo-identity-service > /dev/null 2>&1; then
        die "Container 'margo-identity-service' does not exist. Is MIS installed?"
    fi

    if docker restart margo-identity-service; then
        success "margo-identity-service restarted successfully."

        # Brief wait then status check
        sleep 3
        if docker ps --filter "name=margo-identity-service" \
                     --filter "status=running" \
                     --format "{{.Names}}" | grep -q "margo-identity-service"; then
            success "Container is running."
        else
            warn "Container may not have started cleanly."
            warn "Check: docker logs margo-identity-service"
        fi
    else
        die "Failed to restart margo-identity-service."
    fi
}

# =============================================================================
# ENTRYPOINT
# =============================================================================
main() {
    if [[ $# -eq 0 ]]; then
        error "No flag provided."
        echo ""
        cmd_help
        exit 1
    fi

    case "$1" in
        --install)      cmd_install ;;
        --uninstall)    cmd_uninstall ;;
        --restart-mis)  cmd_restart_mis ;;
        --status)       cmd_status ;;        # ← add this line
        --help|-h)      cmd_help ;;
        *)
            error "Unknown flag: $1"
            echo ""
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"