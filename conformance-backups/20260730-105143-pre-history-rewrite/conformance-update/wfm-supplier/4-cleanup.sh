#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

DATA_DIR="newman-data"
CONTAINERS_FILE="$DATA_DIR/deployed-containers.txt"
EXECUTION_LOG="$DATA_DIR/execution.log"

cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}===================================================${NC}"
echo -e "${BLUE}  WFM Supplier: Cleanup Phase${NC}"
echo -e "${BLUE}===================================================${NC}"
echo ""

ensure_cmd() {
    local cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo -e "${RED}❌ Missing required command: $cmd${NC}"
        return 1
    fi
    return 0
}

ensure_cmd docker || exit 1

# Stop containers listed in deployment file
if [[ -f "$CONTAINERS_FILE" ]]; then
    echo -e "${BLUE}Stopping deployed containers...${NC}"
    STOPPED=0
    FAILED=0
    
    while IFS= read -r container_id; do
        if [[ -n "$container_id" ]]; then
            echo -n "  Stopping $container_id... "
            if docker stop "$container_id" >/dev/null 2>&1; then
                echo -e "${GREEN}✓${NC}"
                ((STOPPED++))
            else
                echo -e "${YELLOW}(already stopped or not found)${NC}"
                ((FAILED++))
            fi
        fi
    done < "$CONTAINERS_FILE"
    
    echo ""
    echo -e "${GREEN}✅ Stopped $STOPPED container(s)${NC}"
    
    # Clear the file
    rm -f "$CONTAINERS_FILE"
else
    echo -e "${YELLOW}⚠️  No deployment tracking file found${NC}"
fi

# Also stop any remaining device-agent labeled containers
echo ""
echo -e "${BLUE}Cleaning up any remaining device-agent containers...${NC}"
REMAINING=$(docker ps -a --filter "label=device-agent" --format "{{.ID}}" | wc -l)

if [[ $REMAINING -gt 0 ]]; then
    docker ps -a --filter "label=device-agent" --format "table {{.ID}}\t{{.Image}}\t{{.Names}}\t{{.Status}}"
    echo ""
    
    read -r -p "Remove these containers? (y/N) " response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        docker rm -f $(docker ps -a --filter "label=device-agent" --format "{{.ID}}") >/dev/null 2>&1
        echo -e "${GREEN}✅ Removed device-agent containers${NC}"
    else
        echo -e "${YELLOW}⚠️  Skipped removal${NC}"
    fi
else
    echo -e "${GREEN}✓ No remaining device-agent containers${NC}"
fi

echo ""
echo -e "${BLUE}===================================================${NC}"
echo -e "${GREEN}✅ Cleanup complete${NC}"
echo -e "${BLUE}===================================================${NC}"
