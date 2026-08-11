#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

echo "Stopping containers..."
docker compose -f "$COMPOSE_FILE" down --volumes --remove-orphans 2>/dev/null || true

echo "Removing chaincode containers..."
docker ps -a --filter "label=org.hyperledger.fabric.chaincode.id" -q 2>/dev/null | xargs docker rm -f 2>/dev/null || true

echo "Removing chaincode images..."
docker images --filter "label=org.hyperledger.fabric.chaincode.id" -q 2>/dev/null | xargs docker rmi -f 2>/dev/null || true

echo "Network stopped."
