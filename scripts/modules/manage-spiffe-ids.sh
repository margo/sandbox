#!/bin/bash
# manage-spiffe-ids.sh
# Manages SPIFFE IDs in a JSON allowlist file.
# Can be sourced as a module or executed standalone.

# ----------------------------
# SPIFFE ID Manager
# ----------------------------

_manage_spiffe_ids_menu() {
  local default_path="${1:-}"
  local role="${2:-}"  # optional: "wfm" or "wfmclient"
  local json_file=""

  # --- Step 1: Resolve file path ---
  if [[ -n "$default_path" ]]; then
    echo ""
    echo "📄 Default SPIFFE ID allowlist path: $default_path"
    read -rp "   Use default path? [Y/n]: " use_default
    use_default="${use_default:-Y}"

    if [[ "$use_default" =~ ^[Yy]$ ]]; then
      json_file="$default_path"
    else
      read -rp "📂 Enter absolute path to SPIFFE ID JSON file: " json_file
    fi
  else
    read -rp "📂 Enter absolute path to SPIFFE ID JSON file: " json_file
  fi

  # Trim whitespace
  json_file="$(echo "$json_file" | xargs)"

  if [[ -z "$json_file" ]]; then
    echo "[ERROR] ❌ No file path provided."
    return 1
  fi

  # Create file with empty array if it doesn't exist
  if [[ ! -f "$json_file" ]]; then
    echo "[INFO] 📝 File not found. Creating new file at: $json_file"
    mkdir -p "$(dirname "$json_file")"
    echo "[]" > "$json_file"
  fi

  # Validate it's a valid JSON array
  if ! jq -e 'if type == "array" then true else error end' "$json_file" > /dev/null 2>&1; then
    echo "[ERROR] ❌ File is not a valid JSON array: $json_file"
    return 1
  fi

  # --- Step 2: Display contents and show options ---
  while true; do
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📋 Current SPIFFE IDs in: $json_file"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    local count
    count=$(jq 'length' "$json_file")

    if [[ "$count" -eq 0 ]]; then
      echo "   (empty — no SPIFFE IDs configured)"
    else
      jq -r 'to_entries[] | "   \(.key + 1). \(.value)"' "$json_file"
    fi

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    if [[ "$role" == "wfm" ]]; then
      echo "What would you like to do?"
      echo "  1) ➕ Add SPIFFE ID"
      echo "  2) ✏️  Edit SPIFFE ID"
      echo "  3) 🔙 Back"
      echo ""
      read -rp "Enter choice [1-3]: " action

      case "$action" in
        1) _spiffe_add_entries "$json_file" "$role" ;;
        2) _spiffe_edit_entry "$json_file" ;;
        3)
          echo "[INFO] 🔙 Returning to previous menu."
          return 0
          ;;
        *)
          echo "[WARN] ⚠️  Invalid choice. Please enter 1, 2, or 3."
          ;;
      esac
    else
      echo "What would you like to do?"
      echo "  1) ➕ Add SPIFFE IDs"
      echo "  2) 🗑️  Remove SPIFFE IDs"
      echo "  3) 🔙 Back"
      echo ""
      read -rp "Enter choice [1-3]: " action

      case "$action" in
        1) _spiffe_add_entries "$json_file" "$role" ;;
        2) _spiffe_remove_entries "$json_file" "$role" ;;
        3)
          echo "[INFO] 🔙 Returning to previous menu."
          return 0
          ;;
        *)
          echo "[WARN] ⚠️  Invalid choice. Please enter 1, 2, or 3."
          ;;
      esac
    fi
  done
}

# --- Add entries ---
_spiffe_add_entries() {
  local json_file="$1"
  local role="${2:-}"

  echo ""
  echo "➕ Enter SPIFFE IDs to add (space-separated):"

  case "$role" in
    wfm)
      echo "   Example: spiffe://margo.org/margo/wfm/symphony-1"
      echo "   💡 Tip: Only one WFM SPIFFE ID is expected. Adding multiple is discouraged."
      ;;
    wfmclient)
      echo "   Example: spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1  spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-2"
      echo "   💡 Tip: Multiple WFM client SPIFFE IDs are supported and encouraged."
      ;;
    *)
      echo "   Example: spiffe://margo.org/margo/wfm/symphony-1"
      ;;
  esac

  read -rp "   > " -a new_ids

  if [[ "${#new_ids[@]}" -eq 0 ]]; then
    echo "[WARN] ⚠️  No SPIFFE IDs provided. Nothing to add."
    return 0
  fi

  # Role-specific warning after input is collected
  if [[ "$role" == "wfm" && "${#new_ids[@]}" -gt 1 ]]; then
    echo "[WARN] ⚠️  Multiple WFM SPIFFE IDs provided. Only a single WFM entry is recommended."
  fi

  # Role-specific warning if a wfm entry already exists
  if [[ "$role" == "wfm" ]]; then
    local existing_count
    existing_count=$(jq 'length' "$json_file")
    if [[ "$existing_count" -gt 0 ]]; then
      echo "[WARN] ⚠️  A WFM SPIFFE ID already exists. Consider using 'Edit' instead of adding another."
    fi
  fi

  local added_count=0
  local skipped_count=0

  # Deduplicate the input list itself first
  local -A seen_input
  local deduped_ids=()
  for sid in "${new_ids[@]}"; do
    sid="$(echo "$sid" | xargs)"  # trim whitespace
    [[ -z "$sid" ]] && continue
    if [[ -z "${seen_input[$sid]+_}" ]]; then
      seen_input[$sid]=1
      deduped_ids+=("$sid")
    fi
  done

  for sid in "${deduped_ids[@]}"; do
    # Check if already exists in file
    local exists
    exists=$(jq --arg id "$sid" 'map(select(. == $id)) | length' "$json_file")

    if [[ "$exists" -gt 0 ]]; then
      echo "[INFO] ⏭️  Skipped (already exists): $sid"
      ((skipped_count++)) || true
    else
      # Append to array
      local tmp
      tmp=$(jq --arg id "$sid" '. += [$id]' "$json_file")
      echo "$tmp" > "$json_file"
      echo "[INFO] ✅ Added:   $sid"
      ((added_count++)) || true
    fi
  done

  echo ""
  echo "[INFO] 📊 Summary — Added: $added_count | Skipped (duplicates): $skipped_count"
}

# --- Edit entry (wfm role only) ---
_spiffe_edit_entry() {
  local json_file="$1"

  local count
  count=$(jq 'length' "$json_file")

  if [[ "$count" -eq 0 ]]; then
    echo "[WARN] ⚠️  No SPIFFE ID exists to edit. Please add one first."
    return 0
  fi

  local current_id
  current_id=$(jq -r '.[0]' "$json_file")

  echo ""
  echo "✏️  Current WFM SPIFFE ID: $current_id"
  echo "   Example: spiffe://margo.org/margo/wfm/symphony-1"
  echo "   💡 Tip: Only one WFM SPIFFE ID is expected. The existing entry will be replaced."
  read -rp "   Enter new SPIFFE ID: " new_id

  new_id="$(echo "$new_id" | xargs)"  # trim whitespace

  if [[ -z "$new_id" ]]; then
    echo "[WARN] ⚠️  No SPIFFE ID provided. Nothing changed."
    return 0
  fi

  if [[ "$new_id" == "$current_id" ]]; then
    echo "[INFO] ⏭️  New SPIFFE ID is the same as the current one. Nothing changed."
    return 0
  fi

  # Replace the first (and expected only) entry
  local tmp
  tmp=$(jq --arg id "$new_id" '[.[1:] | .[] ] | [$id] + .' "$json_file")
  echo "$tmp" > "$json_file"
  echo "[INFO] ✅ Updated SPIFFE ID:"
  echo "         Old: $current_id"
  echo "         New: $new_id"
}

# --- Remove entries ---
_spiffe_remove_entries() {
  local json_file="$1"
  local role="${2:-}"

  echo ""
  echo "🗑️  Enter SPIFFE IDs to remove (space-separated):"

  case "$role" in
    wfmclient)
      echo "   Example: spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1  spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-2"
      echo "   💡 Tip: You can remove multiple WFM client SPIFFE IDs at once."
      ;;
    *)
      echo "   Example: spiffe://margo.org/margo/wfm/symphony-1"
      ;;
  esac

  read -rp "   > " -a remove_ids

  if [[ "${#remove_ids[@]}" -eq 0 ]]; then
    echo "[WARN] ⚠️  No SPIFFE IDs provided. Nothing to remove."
    return 0
  fi

  local removed_count=0
  local notfound_count=0

  # Deduplicate the removal input list itself first
  local -A seen_input
  local deduped_ids=()
  for sid in "${remove_ids[@]}"; do
    sid="$(echo "$sid" | xargs)"  # trim whitespace
    [[ -z "$sid" ]] && continue
    if [[ -z "${seen_input[$sid]+_}" ]]; then
      seen_input[$sid]=1
      deduped_ids+=("$sid")
    fi
  done

  for sid in "${deduped_ids[@]}"; do
    # Check if exists in file
    local exists
    exists=$(jq --arg id "$sid" 'map(select(. == $id)) | length' "$json_file")

    if [[ "$exists" -eq 0 ]]; then
      echo "[INFO] ⏭️  No-op (not found): $sid"
      ((notfound_count++)) || true
    else
      local tmp
      tmp=$(jq --arg id "$sid" 'map(select(. != $id))' "$json_file")
      echo "$tmp" > "$json_file"
      echo "[INFO] 🗑️  Removed:   $sid"
      ((removed_count++)) || true
    fi
  done

  echo ""
  echo "[INFO] 📊 Summary — Removed: $removed_count | Not found (no-op): $notfound_count"
}