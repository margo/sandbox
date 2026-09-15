#!/usr/bin/env bash

set -euo pipefail

OUTPUT_FILE="labels.json"

print_header() {
    echo
    echo "========================================"
    echo "        Labels JSON Builder"
    echo "========================================"
    echo "Output file: $OUTPUT_FILE"
    echo
}

initialize_json_file() {
    if [[ ! -f "$OUTPUT_FILE" || ! -s "$OUTPUT_FILE" ]]; then
        echo "{}" > "$OUTPUT_FILE"
        return
    fi

    if ! python3 -m json.tool "$OUTPUT_FILE" > /dev/null 2>&1; then
        echo "Warning: $OUTPUT_FILE exists but is not valid JSON."
        read -rp "Do you want to overwrite it with an empty JSON object? [y/N]: " overwrite
        case "$overwrite" in
            y|Y|yes|YES)
                echo "{}" > "$OUTPUT_FILE"
                ;;
            *)
                echo "Cannot continue with invalid JSON. Exiting."
                exit 1
                ;;
        esac
    fi
}

show_main_menu() {
    echo
    echo "What would you like to do?"
    echo "1) Add a new key-value pair"
    echo "2) View current labels.json"
    echo "3) Exit"
    echo
}

show_value_types() {
    echo
    echo "Select the value type:"
    echo "1) String"
    echo "2) Number"
    echo "3) Boolean"
    echo "4) Array of strings"
    echo "5) Array of numbers"
    echo
}

validate_number() {
    local value="$1"
    [[ "$value" =~ ^-?([0-9]+)([.][0-9]+)?$ ]]
}

validate_boolean() {
    local value="$1"
    [[ "$value" == "true" || "$value" == "false" ]]
}

warn_if_no_domain_prefix() {
    local key="$1"

    if [[ ! "$key" =~ ^[A-Za-z0-9.-]+/[^/]+$ ]]; then
        echo
        echo "Note: Prefixing with an organization domain is recommended for supplier-specific labels."
        echo "Example: example.com/$key"
        echo
    fi
}

read_non_empty() {
    local prompt="$1"
    local input=""

    while true; do
        read -rp "$prompt" input
        if [[ -n "$input" ]]; then
            printf '%s' "$input"
            return 0
        fi
        echo "Input cannot be empty. Please try again."
    done
}

add_to_json() {
    local key="$1"
    local type="$2"
    local value="$3"

    python3 - "$OUTPUT_FILE" "$key" "$type" "$value" <<'PY'
import json
import sys
from pathlib import Path

file_path = Path(sys.argv[1])
key = sys.argv[2]
value_type = sys.argv[3]
raw_value = sys.argv[4]

try:
    with file_path.open("r", encoding="utf-8") as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    data = {}

if not isinstance(data, dict):
    print("Error: labels.json must contain a JSON object at the top level.", file=sys.stderr)
    sys.exit(1)

try:
    if value_type == "string":
        value = raw_value

    elif value_type == "number":
        value = float(raw_value) if "." in raw_value else int(raw_value)

    elif value_type == "boolean":
        value = raw_value.lower() == "true"

    elif value_type == "array_strings":
        value = raw_value.split()
        if not value:
            raise ValueError("Array must contain at least one string.")

    elif value_type == "array_numbers":
        value = []
        for item in raw_value.split():
            value.append(float(item) if "." in item else int(item))
        if not value:
            raise ValueError("Array must contain at least one number.")

    else:
        raise ValueError("Unsupported value type.")

except ValueError as exc:
    print(f"Error: {exc}", file=sys.stderr)
    sys.exit(1)

data[key] = value

with file_path.open("w", encoding="utf-8") as f:
    json.dump(data, f, indent=4)
    f.write("\n")

print(f"Successfully added/updated key: {key}")
PY
}

handle_add_label() {
    local key=""
    local type_choice=""
    local value=""
    local valid_array=true
    local item=""

    echo
    key=$(read_non_empty "Enter the label key: ")
    warn_if_no_domain_prefix "$key"

    show_value_types
    read -rp "Enter value type choice [1-5]: " type_choice

    case "$type_choice" in
        1)
            value=$(read_non_empty "Enter string value: ")
            add_to_json "$key" "string" "$value"
            ;;

        2)
            while true; do
                value=$(read_non_empty "Enter number value: ")
                if validate_number "$value"; then
                    break
                fi
                echo "Invalid number. Please enter a valid integer or decimal number."
            done
            add_to_json "$key" "number" "$value"
            ;;

        3)
            while true; do
                value=$(read_non_empty "Enter boolean value [true/false]: ")
                if validate_boolean "$value"; then
                    break
                fi
                echo "Invalid boolean. Please enter only true or false in lowercase."
            done
            add_to_json "$key" "boolean" "$value"
            ;;

        4)
            value=$(read_non_empty "Enter array string values separated by spaces: ")
            add_to_json "$key" "array_strings" "$value"
            ;;

        5)
            while true; do
                value=$(read_non_empty "Enter array number values separated by spaces: ")
                valid_array=true

                for item in $value; do
                    if ! validate_number "$item"; then
                        valid_array=false
                        break
                    fi
                done

                if [[ "$valid_array" == "true" ]]; then
                    break
                fi

                echo "Invalid array. All values must be valid numbers."
            done
            add_to_json "$key" "array_numbers" "$value"
            ;;

        *)
            echo "Invalid value type choice. Please select a number from 1 to 5."
            return
            ;;
    esac

    echo
    echo "Current contents of $OUTPUT_FILE:"
    python3 -m json.tool "$OUTPUT_FILE"
}

main() {
    local choice=""

    print_header
    initialize_json_file

    while true; do
        show_main_menu
        read -rp "Enter your choice [1-3]: " choice

        case "$choice" in
            1)
                handle_add_label
                ;;
            2)
                echo
                echo "Current contents of $OUTPUT_FILE:"
                python3 -m json.tool "$OUTPUT_FILE"
                ;;
            3)
                echo
                echo "Exiting. Final $OUTPUT_FILE:"
                python3 -m json.tool "$OUTPUT_FILE"
                echo
                exit 0
                ;;
            *)
                echo "Invalid choice. Please select 1, 2, or 3."
                ;;
        esac
    done
}

main "$@"
